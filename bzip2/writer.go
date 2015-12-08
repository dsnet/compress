// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"io"

	"github.com/dsnet/compress/internal"
	"github.com/dsnet/compress/internal/prefix"
)

type Writer struct {
	InputOffset  int64 // Total number of bytes issued to Write
	OutputOffset int64 // Total number of bytes written to underlying io.Writer

	wr     prefixWriter
	err    error
	level  int
	wrHdr  bool
	blkCRC uint32
	endCRC uint32

	rle runLengthEncoding
	bwt burrowsWheelerTransform
	mtf moveToFront

	buf []byte
}

func NewWriter(w io.Writer) *Writer {
	zw, _ := NewWriterLevel(w, DefaultCompression)
	return zw
}

func NewWriterLevel(w io.Writer, lvl int) (*Writer, error) {
	if lvl == DefaultCompression {
		lvl = 6
	}
	if lvl < BestSpeed || lvl > BestCompression {
		return nil, Error{"invalid compression level"}
	}
	zw := new(Writer)
	zw.level = lvl
	zw.Reset(w)
	return zw, nil
}

func (zw *Writer) Write(buf []byte) (int, error) {
	if zw.err != nil {
		return 0, zw.err
	}

	cnt := len(buf)
	for len(buf) > 0 {
		wrCnt, err := zw.rle.Write(buf)
		zw.blkCRC = updateCRC(zw.blkCRC, buf[:wrCnt])
		buf = buf[wrCnt:]
		if err != rleDone {
			continue
		}
		if zw.err = zw.flush(); zw.err != nil {
			return 0, zw.err
		}
	}
	zw.InputOffset += int64(cnt)
	return cnt, nil
}

func (zw *Writer) flush() error {
	vals := zw.rle.Bytes()
	if len(vals) == 0 {
		return nil
	}
	zw.wr.Offset = zw.OutputOffset
	func() {
		defer errRecover(&zw.err)
		if !zw.wrHdr {
			// Write stream header.
			zw.wr.WriteBitsBE64(hdrMagic, 16)
			zw.wr.WriteBitsBE64('h', 8)
			zw.wr.WriteBitsBE64(uint64('0'+zw.level), 8)
			zw.wrHdr = true
		}
		zw.compressBlock(vals)
	}()
	if zw.OutputOffset, zw.err = zw.wr.Flush(); zw.err != nil {
		return zw.err
	}
	zw.endCRC = (zw.endCRC<<1 | zw.endCRC>>31) ^ zw.blkCRC
	zw.blkCRC = 0
	zw.rle.Init(zw.buf)
	return nil
}

func (zw *Writer) Close() error {
	if zw.err == ErrClosed {
		return nil
	}

	// Flush RLE buffer if there is left-over data.
	if zw.err = zw.flush(); zw.err != nil {
		return zw.err
	}

	// Write stream footer.
	zw.wr.Offset = zw.OutputOffset
	func() {
		defer errRecover(&zw.err)
		zw.wr.WriteBitsBE64(endMagic, 48)
		zw.wr.WriteBitsBE64(uint64(zw.endCRC), 32)
		zw.wr.WritePads(0)
	}()
	if zw.OutputOffset, zw.err = zw.wr.Flush(); zw.err != nil {
		return zw.err
	}

	if zw.err == nil {
		zw.err = ErrClosed
		return nil
	}
	return zw.err
}

func (zw *Writer) Reset(w io.Writer) {
	*zw = Writer{
		wr:    zw.wr,
		level: zw.level,
		rle:   zw.rle,
		bwt:   zw.bwt,
		mtf:   zw.mtf,
		buf:   zw.buf,
	}
	zw.wr.Init(w)
	blkSize := zw.level * blockSize
	if cap(zw.buf) >= blkSize {
		zw.buf = zw.buf[:blkSize]
	} else {
		zw.buf = make([]byte, blkSize)
	}
	zw.rle.Init(zw.buf)
	return
}

func (zw *Writer) compressBlock(buf []byte) {
	zw.wr.WriteBitsBE64(blkMagic, 48)
	zw.wr.WriteBitsBE64(uint64(zw.blkCRC), 32)
	zw.wr.WriteBitsBE64(0, 1)

	// Step 1: Burrows-Wheeler transformation.
	ptr := zw.bwt.Encode(buf)
	zw.wr.WriteBitsBE64(uint64(ptr), 24)

	// Step 2: Move-to-front transform and run-length encoding.
	var dictMap [256]bool
	for _, c := range buf {
		dictMap[c] = true
	}

	var dictArr [256]uint8
	var symMaps [16]uint16
	var symMap uint16
	dict := dictArr[:0]
	for i, b := range dictMap {
		if b {
			c := uint8(i)
			dict = append(dict, c)
			symMap |= 1 << (c >> 4)
			symMaps[c>>4] |= 1 << (c & 0xf)
		}
	}

	zw.wr.WriteBits(uint(symMap), 16)
	for _, m := range symMaps {
		if m > 0 {
			zw.wr.WriteBits(uint(m), 16)
		}
	}

	zw.mtf.Init(dict, len(buf))
	syms := zw.mtf.Encode(buf)

	// Step 3: Prefix encoding.
	zw.encodePrefix(syms, len(dict))
}

func (zw *Writer) encodePrefix(syms []uint16, numSyms int) {
	numSyms += 2                           // Remove 0 symbol, add RUNA, RUNB, and EOF symbols
	syms = append(syms, uint16(numSyms-1)) // EOF marker

	// Compute number of prefix trees needed.
	numTrees := maxNumTrees
	for i, lim := range []int{200, 600, 1200, 2400} {
		if len(syms) < lim {
			numTrees = minNumTrees + i
			break
		}
	}

	// Compute number of block selectors.
	numSels := (len(syms) + numBlockSyms - 1) / numBlockSyms
	treeSels := make([]uint8, numSels)
	for i := range treeSels {
		treeSels[i] = uint8(i % numTrees)
	}

	// Initialize prefix codes.
	var codes2D [maxNumTrees][maxNumSyms]prefix.PrefixCode
	var codes1D [maxNumTrees]prefix.PrefixCodes
	var trees1D [maxNumTrees]prefix.Encoder
	for i := range codes2D[:numTrees] {
		pc := codes2D[i][:numSyms]
		for j := range pc {
			pc[j].Sym = uint32(j)
		}
		codes1D[i] = pc
	}

	// First cut at assigning prefix trees to each group.
	var codes prefix.PrefixCodes
	var blkLen, treeIdx int
	for _, sym := range syms {
		if blkLen == 0 {
			blkLen = numBlockSyms
			codes = codes2D[treeSels[treeIdx]][:numSyms]
			treeIdx++
		}
		blkLen--
		codes[sym].Cnt++
	}

	// TODO(dsnet): Use K-means to cluster groups to each prefix tree.

	// Generate lengths and prefixes based on symbol frequencies.
	for i := range trees1D[:numTrees] {
		pc := prefix.PrefixCodes(codes2D[i][:numSyms])
		pc.SortByCount()
		if err := prefix.GenerateLengths(pc, maxPrefixBits); err != nil {
			panic(err)
		}
		pc.SortBySymbol()
		if err := prefix.GeneratePrefixes(pc); err != nil {
			panic(err)
		}
		trees1D[i].Init(pc)
	}

	// Write out information about the trees and tree selectors.
	var mtf internal.MoveToFront
	zw.wr.WriteBitsBE64(uint64(numTrees), 3)
	zw.wr.WriteBitsBE64(uint64(numSels), 15)
	treeSelsMTF := make([]uint8, numSels)
	copy(treeSelsMTF, treeSels)
	mtf.Encode(treeSelsMTF)
	for _, sym := range treeSelsMTF {
		zw.wr.WriteSymbol(uint(sym), &encSel)
	}
	zw.wr.WritePrefixCodes(codes1D[:numTrees])

	// Write out prefix encoded symbols of compressed data.
	var tree *prefix.Encoder
	blkLen, treeIdx = 0, 0
	for _, sym := range syms {
		if blkLen == 0 {
			blkLen = numBlockSyms
			tree = &trees1D[treeSels[treeIdx]]
			treeIdx++
		}
		blkLen--
		ok := zw.wr.TryWriteSymbol(uint(sym), tree)
		if !ok {
			zw.wr.WriteSymbol(uint(sym), tree)
		}
	}
}
