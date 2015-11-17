// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import "io"

type Reader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read

	rd     bitReader // Input source
	toRead []byte    // Uncompressed data ready to be emitted from Read
	dist   int       // The current distance
	blkLen int       // Uncompressed bytes left to read in meta-block
	cpyLen int       // Bytes left to backward dictionary copy
	last   bool      // Last block bit detected
	err    error     // Persistent error

	step      func(*Reader) // Single step of decompression work (can panic)
	stepState int           // The sub-step state for certain steps

	dict     dictDecoder   // Dynamic sliding dictionary
	litTree  prefixDecoder // Literal and length symbol prefix decoder
	distTree prefixDecoder // Backward distance symbol prefix decoder
}

func NewReader(r io.Reader) *Reader {
	fr := new(Reader)
	fr.Reset(r)
	return fr
}

func (fr *Reader) Read(buf []byte) (int, error) {
	for {
		if len(fr.toRead) > 0 {
			cnt := copy(buf, fr.toRead)
			fr.toRead = fr.toRead[cnt:]
			fr.OutputOffset += int64(cnt)
			return cnt, nil
		}
		if fr.err != nil {
			return 0, fr.err
		}

		// Perform next step in decompression process.
		fr.rd.offset = fr.InputOffset
		func() {
			defer errRecover(&fr.err)
			fr.step(fr)
		}()
		fr.InputOffset = fr.rd.FlushOffset()
		if fr.err != nil {
			fr.toRead = fr.dict.ReadFlush() // Flush what's left in case of error
		}
	}
}

func (fr *Reader) Close() error {
	if fr.err == io.EOF || fr.err == io.ErrClosedPipe {
		fr.toRead = nil // Make sure future reads fail
		fr.err = io.ErrClosedPipe
		return nil
	}
	return fr.err // Return the persistent error
}

func (fr *Reader) Reset(r io.Reader) error {
	*fr = Reader{
		rd:   fr.rd,
		step: (*Reader).readBlockHeader,
		dict: fr.dict,
	}
	fr.rd.Init(r)
	fr.dict.Init(maxHistSize)
	return nil
}

// readBlockHeader reads the block header according to RFC section 3.2.3.
func (fr *Reader) readBlockHeader() {
	if fr.last {
		fr.rd.ReadPads()
		panic(io.EOF)
	}

	fr.last = fr.rd.ReadBits(1) == 1
	switch fr.rd.ReadBits(2) {
	case 0:
		// Raw block (RFC section 3.2.4).
		fr.rd.ReadPads()

		n := uint16(fr.rd.ReadBits(16))
		nn := uint16(fr.rd.ReadBits(16))
		if n^nn != 0xffff {
			panic(ErrCorrupt)
		}
		fr.blkLen = int(n)

		// By convention, an empty block flushes the read buffer.
		if fr.blkLen == 0 {
			fr.toRead = fr.dict.ReadFlush()
			fr.step = (*Reader).readBlockHeader
			return
		}
		fr.step = (*Reader).readRawData
	case 1:
		// Fixed prefix block (RFC section 3.2.6).
		fr.litTree, fr.distTree = litTree, distTree
		fr.step = (*Reader).readBlock
	case 2:
		// Dynamic prefix block (RFC section 3.2.7).
		fr.rd.ReadPrefixCodes(&fr.litTree, &fr.distTree)
		fr.step = (*Reader).readBlock
	default:
		// Reserved block (RFC section 3.2.3).
		panic(ErrCorrupt)
	}
}

// readRawData reads raw data according to RFC section 3.2.4.
func (fr *Reader) readRawData() {
	buf := fr.dict.WriteSlice()
	if len(buf) > fr.blkLen {
		buf = buf[:fr.blkLen]
	}

	cnt, err := fr.rd.Read(buf)
	fr.blkLen -= cnt
	fr.dict.WriteMark(cnt)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		panic(err)
	}

	if fr.blkLen > 0 {
		fr.toRead = fr.dict.ReadFlush()
		fr.step = (*Reader).readRawData // We need to continue this work
		return
	}
	fr.step = (*Reader).readBlockHeader
}

// readCommands reads block commands according to RFC section 3.2.3.
func (fr *Reader) readBlock() {
	const (
		stateInit = iota // Zero value must be stateInit
		stateDict
	)

	switch fr.stepState {
	case stateInit:
		goto readLiteral
	case stateDict:
		goto copyDistance
	}

readLiteral:
	// Read literal and/or (length, distance) according to RFC section 3.2.3.
	{
		if fr.dict.AvailSize() == 0 {
			fr.toRead = fr.dict.ReadFlush()
			fr.step = (*Reader).readBlock
			fr.stepState = stateInit // Need to continue work here
			return
		}

		// Read the literal symbol.
		litSym, ok := fr.rd.TryReadSymbol(&fr.litTree)
		if !ok {
			litSym = fr.rd.ReadSymbol(&fr.litTree)
		}
		switch {
		case litSym < endBlockSym:
			fr.dict.WriteByte(byte(litSym))
			goto readLiteral
		case litSym == endBlockSym:
			fr.step = (*Reader).readBlockHeader
			fr.stepState = stateInit // Next call to readBlock must start here
			return
		case litSym < maxNumLitSyms:
			// Decode the copy length.
			rec := lenLUT[litSym-257]
			extra, ok := fr.rd.TryReadBits(uint(rec.bits))
			if !ok {
				extra = fr.rd.ReadBits(uint(rec.bits))
			}
			fr.cpyLen = int(rec.base) + int(extra)

			// Read the distance symbol.
			distSym, ok := fr.rd.TryReadSymbol(&fr.distTree)
			if !ok {
				distSym = fr.rd.ReadSymbol(&fr.distTree)
			}
			if distSym >= maxNumDistSyms {
				panic(ErrCorrupt)
			}

			// Decode the copy distance.
			rec = distLUT[distSym]
			extra, ok = fr.rd.TryReadBits(uint(rec.bits))
			if !ok {
				extra = fr.rd.ReadBits(uint(rec.bits))
			}
			fr.dist = int(rec.base) + int(extra)

			goto copyDistance
		default:
			panic(ErrCorrupt)
		}
	}

copyDistance:
	// Perform a backwards copy according to RFC section 3.2.3.
	{
		cnt := fr.dict.WriteCopy(fr.dist, fr.cpyLen)
		fr.cpyLen -= cnt

		if fr.cpyLen > 0 {
			fr.toRead = fr.dict.ReadFlush()
			fr.step = (*Reader).readBlock
			fr.stepState = stateDict // Need to continue work here
			return
		} else {
			goto readLiteral
		}
	}
}
