// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import (
	"io"

	"github.com/dsnet/compress/internal"
	"github.com/dsnet/compress/internal/prefix"
)

type Reader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read

	rd     prefixReader // Input source
	toRead []byte       // Uncompressed data ready to be emitted from Read
	dist   int          // The current distance
	blkLen int          // Uncompressed bytes left to read in meta-block
	cpyLen int          // Bytes left to backward dictionary copy
	last   bool         // Last block bit detected
	err    error        // Persistent error

	step      func(*Reader) // Single step of decompression work (can panic)
	stepState int           // The sub-step state for certain steps

	dict     dictDecoder    // Dynamic sliding dictionary
	litTree  prefix.Decoder // Literal and length symbol prefix decoder
	distTree prefix.Decoder // Backward distance symbol prefix decoder
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
		rd:   zr.rd,
		step: (*Reader).readBlockHeader,
		dict: zr.dict,
	}
	zr.rd.Init(r)
	zr.dict.Init(maxHistSize)
}

func (zr *Reader) Read(buf []byte) (int, error) {
	for {
		if len(zr.toRead) > 0 {
			cnt := copy(buf, zr.toRead)
			zr.toRead = zr.toRead[cnt:]
			zr.OutputOffset += int64(cnt)
			return cnt, nil
		}
		if zr.err != nil {
			return 0, zr.err
		}

		// Perform next step in decompression process.
		zr.rd.Offset = zr.InputOffset
		func() {
			defer errRecover(&zr.err)
			zr.step(zr)
		}()
		var err error
		if zr.InputOffset, err = zr.rd.Flush(); err != nil {
			zr.err = err
		}
		if zr.err != nil {
			if zr.err == internal.ErrInvalid {
				zr.err = ErrCorrupt
			}
		}
		if zr.err != nil {
			zr.toRead = zr.dict.ReadFlush() // Flush what's left in case of error
		}
	}
}

func (zr *Reader) Close() error {
	zr.toRead = nil // Make sure future reads fail
	if zr.err == io.EOF || zr.err == ErrClosed {
		zr.err = ErrClosed
		return nil
	}
	return zr.err // Return the persistent error
}

// readBlockHeader reads the block header according to RFC section 3.2.3.
func (zr *Reader) readBlockHeader() {
	if zr.last {
		zr.rd.ReadPads()
		panic(io.EOF)
	}

	zr.last = zr.rd.ReadBits(1) == 1
	switch zr.rd.ReadBits(2) {
	case 0:
		// Raw block (RFC section 3.2.4).
		zr.rd.ReadPads()

		n := uint16(zr.rd.ReadBits(16))
		nn := uint16(zr.rd.ReadBits(16))
		if n^nn != 0xffff {
			panic(ErrCorrupt)
		}
		zr.blkLen = int(n)

		// By convention, an empty block flushes the read buffer.
		if zr.blkLen == 0 {
			zr.toRead = zr.dict.ReadFlush()
			zr.step = (*Reader).readBlockHeader
			return
		}
		zr.step = (*Reader).readRawData
	case 1:
		// Fixed prefix block (RFC section 3.2.6).
		zr.litTree, zr.distTree = decLit, decDist
		zr.step = (*Reader).readBlock
	case 2:
		// Dynamic prefix block (RFC section 3.2.7).
		zr.rd.ReadPrefixCodes(&zr.litTree, &zr.distTree)
		zr.step = (*Reader).readBlock
	default:
		// Reserved block (RFC section 3.2.3).
		panic(ErrCorrupt)
	}
}

// readRawData reads raw data according to RFC section 3.2.4.
func (zr *Reader) readRawData() {
	buf := zr.dict.WriteSlice()
	if len(buf) > zr.blkLen {
		buf = buf[:zr.blkLen]
	}

	cnt, err := zr.rd.Read(buf)
	zr.blkLen -= cnt
	zr.dict.WriteMark(cnt)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		panic(err)
	}

	if zr.blkLen > 0 {
		zr.toRead = zr.dict.ReadFlush()
		zr.step = (*Reader).readRawData // We need to continue this work
		return
	}
	zr.step = (*Reader).readBlockHeader
}

// readCommands reads block commands according to RFC section 3.2.3.
func (zr *Reader) readBlock() {
	const (
		stateInit = iota // Zero value must be stateInit
		stateDict
	)

	switch zr.stepState {
	case stateInit:
		goto readLiteral
	case stateDict:
		goto copyDistance
	}

readLiteral:
	// Read literal and/or (length, distance) according to RFC section 3.2.3.
	{
		if zr.dict.AvailSize() == 0 {
			zr.toRead = zr.dict.ReadFlush()
			zr.step = (*Reader).readBlock
			zr.stepState = stateInit // Need to continue work here
			return
		}

		// Read the literal symbol.
		litSym, ok := zr.rd.TryReadSymbol(&zr.litTree)
		if !ok {
			litSym = zr.rd.ReadSymbol(&zr.litTree)
		}
		switch {
		case litSym < endBlockSym:
			zr.dict.WriteByte(byte(litSym))
			goto readLiteral
		case litSym == endBlockSym:
			zr.step = (*Reader).readBlockHeader
			zr.stepState = stateInit // Next call to readBlock must start here
			return
		case litSym < maxNumLitSyms:
			// Decode the copy length.
			rec := lenRanges[litSym-257]
			extra, ok := zr.rd.TryReadBits(uint(rec.Len))
			if !ok {
				extra = zr.rd.ReadBits(uint(rec.Len))
			}
			zr.cpyLen = int(rec.Base) + int(extra)

			// Read the distance symbol.
			distSym, ok := zr.rd.TryReadSymbol(&zr.distTree)
			if !ok {
				distSym = zr.rd.ReadSymbol(&zr.distTree)
			}
			if distSym >= maxNumDistSyms {
				panic(ErrCorrupt)
			}

			// Decode the copy distance.
			rec = distRanges[distSym]
			extra, ok = zr.rd.TryReadBits(uint(rec.Len))
			if !ok {
				extra = zr.rd.ReadBits(uint(rec.Len))
			}
			zr.dist = int(rec.Base) + int(extra)

			goto copyDistance
		default:
			panic(ErrCorrupt)
		}
	}

copyDistance:
	// Perform a backwards copy according to RFC section 3.2.3.
	{
		cnt := zr.dict.TryWriteCopy(zr.dist, zr.cpyLen)
		if cnt == 0 {
			cnt = zr.dict.WriteCopy(zr.dist, zr.cpyLen)
		}
		zr.cpyLen -= cnt

		if zr.cpyLen > 0 {
			zr.toRead = zr.dict.ReadFlush()
			zr.step = (*Reader).readBlock
			zr.stepState = stateDict // Need to continue work here
			return
		} else {
			goto readLiteral
		}
	}
}
