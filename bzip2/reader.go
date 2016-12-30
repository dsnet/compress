// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"io"

	"github.com/dsnet/compress/internal"
	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/prefix"
)

type Reader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read

	rd         prefixReader
	err        error
	level      int    // The current compression level
	rdHdr      bool   // Have we read the stream header?
	gotBlkCRC  uint32 // CRC-32 IEEE of each block (as stored)
	wantBlkCRC uint32 // CRC-32 IEEE of each block (as computed)
	endCRC     uint32 // Checksum of all blocks using bzip2's custom method

	crc crc
	mtf moveToFront
	bwt burrowsWheelerTransform
	rle runLengthEncoding
}

type ReaderConfig struct {
	_ struct{} // Blank field to prevent unkeyed struct literals
}

func NewReader(r io.Reader, conf *ReaderConfig) (*Reader, error) {
	zr := new(Reader)
	zr.Reset(r)
	return zr, nil
}

func (zr *Reader) Reset(r io.Reader) {
	*zr = Reader{
		rd:  zr.rd,
		mtf: zr.mtf,
		bwt: zr.bwt,
		rle: zr.rle,
	}
	zr.rd.Init(r)
	return
}

func (zr *Reader) Read(buf []byte) (int, error) {
	for {
		cnt, _ := zr.rle.Read(buf)
		if cnt > 0 {
			zr.wantBlkCRC = zr.crc.update(zr.wantBlkCRC, buf[:cnt])
			zr.OutputOffset += int64(cnt)
			return cnt, nil
		}
		if zr.err != nil || len(buf) == 0 {
			return 0, zr.err
		}

		// Update the CRC.
		if zr.gotBlkCRC != zr.wantBlkCRC {
			zr.err = errorf(errors.Corrupted, "mismatching block checksum")
			return 0, zr.err
		}
		zr.endCRC = (zr.endCRC<<1 | zr.endCRC>>31) ^ zr.wantBlkCRC
		zr.wantBlkCRC = 0

		// Read the next chunk.
		zr.rd.Offset = zr.InputOffset
		func() {
			defer errors.Recover(&zr.err)
			if !zr.rdHdr {
				// Read stream header.
				if zr.rd.ReadBitsBE64(16) != hdrMagic {
					panicf(errors.Corrupted, "invalid stream magic")
				}
				if ver := zr.rd.ReadBitsBE64(8); ver != 'h' {
					if ver == '0' {
						panicf(errors.Deprecated, "bzip1 format is not supported")
					}
					panicf(errors.Corrupted, "invalid version: %q", ver)
				}
				lvl := int(zr.rd.ReadBitsBE64(8)) - '0'
				if lvl < BestSpeed || lvl > BestCompression {
					panicf(errors.Corrupted, "invalid block size: %d", lvl*blockSize)
				}
				zr.level = lvl
				zr.rdHdr = true
			}
			buf := zr.decodeBlock()
			zr.rle.Init(buf)
		}()
		var err error
		if zr.InputOffset, err = zr.rd.Flush(); zr.err == nil {
			zr.err = err
		}
		if zr.err != nil {
			zr.err = errWrap(zr.err, errors.Corrupted)
			return 0, zr.err
		}
	}
}

func (zr *Reader) Close() error {
	if zr.err == io.EOF || zr.err == errClosed {
		zr.rle.Init(nil) // Make sure future reads fail
		zr.err = errClosed
		return nil
	}
	return zr.err // Return the persistent error
}

func (zr *Reader) decodeBlock() []byte {
	if magic := zr.rd.ReadBitsBE64(48); magic != blkMagic {
		if magic == endMagic {
			// TODO(dsnet): Handle multiple bzip2 files back-to-back.
			if zr.endCRC != uint32(zr.rd.ReadBitsBE64(32)) {
				panicf(errors.Corrupted, "mismatching stream checksum")
			}
			zr.rd.ReadPads()
			errors.Panic(io.EOF)
		}
		panicf(errors.Corrupted, "invalid block or footer magic")
	}
	zr.gotBlkCRC = uint32(zr.rd.ReadBitsBE64(32))
	if zr.rd.ReadBitsBE64(1) != 0 {
		panicf(errors.Deprecated, "block randomization is not supported")
	}

	// Read BWT related fields.
	ptr := int(zr.rd.ReadBitsBE64(24)) // BWT origin pointer

	// Read MTF related fields.
	var dictArr [256]uint8
	dict := dictArr[:0]
	bmapHi := uint16(zr.rd.ReadBits(16))
	for i := 0; i < 256; i, bmapHi = i+16, bmapHi>>1 {
		if bmapHi&1 > 0 {
			bmapLo := uint16(zr.rd.ReadBits(16))
			for j := 0; j < 16; j, bmapLo = j+1, bmapLo>>1 {
				if bmapLo&1 > 0 {
					dict = append(dict, uint8(i+j))
				}
			}
		}
	}

	// Step 1: Prefix encoding.
	syms := zr.decodePrefix(len(dict))

	// Step 2: Move-to-front transform and run-length encoding.
	zr.mtf.Init(dict, zr.level*blockSize)
	buf := zr.mtf.Decode(syms)

	// Step 3: Burrows-Wheeler transformation.
	if ptr >= len(buf) {
		panicf(errors.Corrupted, "origin pointer (0x%06x) exceeds block size: %d", ptr, len(buf))
	}
	zr.bwt.Decode(buf, ptr)

	return buf
}

func (zr *Reader) decodePrefix(numSyms int) (syms []uint16) {
	numSyms += 2 // Remove 0 symbol, add RUNA, RUNB, and EOF symbols
	if numSyms < 3 {
		panicf(errors.Corrupted, "not enough prefix symbols: %d", numSyms)
	}

	// Read information about the trees and tree selectors.
	var mtf internal.MoveToFront
	numTrees := int(zr.rd.ReadBitsBE64(3))
	if numTrees < minNumTrees || numTrees > maxNumTrees {
		panicf(errors.Corrupted, "invalid number of prefix trees: %d", numTrees)
	}
	numSels := int(zr.rd.ReadBitsBE64(15))
	treeSels := make([]uint8, numSels)
	for i := range treeSels {
		sym, ok := zr.rd.TryReadSymbol(&decSel)
		if !ok {
			sym = zr.rd.ReadSymbol(&decSel)
		}
		if int(sym) >= numTrees {
			panicf(errors.Corrupted, "invalid prefix tree selector: %d", sym)
		}
		treeSels[i] = uint8(sym)
	}
	mtf.Decode(treeSels)

	// Initialize prefix codes.
	var codes2D [maxNumTrees][maxNumSyms]prefix.PrefixCode
	var codes1D [maxNumTrees]prefix.PrefixCodes
	var trees1D [maxNumTrees]prefix.Decoder
	for i := range codes2D[:numTrees] {
		pc := codes2D[i][:numSyms]
		for j := range pc {
			pc[j].Sym = uint32(j)
		}
		codes1D[i] = pc
	}
	zr.rd.ReadPrefixCodes(codes1D[:numTrees], trees1D[:numTrees])

	// Read prefix encoded symbols of compressed data.
	var tree *prefix.Decoder
	var blkLen, selIdx int
	for {
		if blkLen == 0 {
			blkLen = numBlockSyms
			if selIdx >= len(treeSels) {
				panicf(errors.Corrupted, "not enough prefix tree selectors")
			}
			tree = &trees1D[treeSels[selIdx]]
			selIdx++
		}
		blkLen--
		sym, ok := zr.rd.TryReadSymbol(tree)
		if !ok {
			sym = zr.rd.ReadSymbol(tree)
		}

		if int(sym) == numSyms-1 {
			break // EOF marker
		}
		if int(sym) >= numSyms {
			panicf(errors.Corrupted, "invalid prefix symbol: %d", sym)
		}
		if len(syms) >= zr.level*blockSize {
			panicf(errors.Corrupted, "number of prefix symbols exceeds block size")
		}
		syms = append(syms, uint16(sym))
	}
	return syms
}
