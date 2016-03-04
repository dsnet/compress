// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/dsnet/compress/internal/prefix"
)

// A Reader is an io.Reader that can read XFLATE's meta encoding.
// The zero value of Reader is valid once Reset is called.
type Reader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read
	NumBlocks    int64 // Number of blocks decoded

	// FinalMode indicates which final bits (if any) were set.
	// This will be valid after a call to Close or upon hitting io.EOF.
	FinalMode FinalMode

	rd *prefix.Reader
	br prefix.Reader // Pre-allocated prefix.Reader to wrap input Reader
	bw prefix.Writer // Temporary bit writer
	bb bytes.Buffer  // Buffer for bw to write into

	final FinalMode
	buf   []byte
	err   error
}

// NewReader creates a new Reader reading from the given reader.
// If rd does not also implement compress.ByteReader or compress.BufferedReader,
// then the decoder may read more data than necessary from rd.
func NewReader(rd io.Reader) *Reader {
	mr := new(Reader)
	mr.Reset(rd)
	return mr
}

// Reset discards the Reader's state and makes it equivalent to the result
// of a call to NewReader, but reading from rd instead.
//
// This is used to reduce memory allocations.
func (mr *Reader) Reset(rd io.Reader) {
	*mr = Reader{
		br: mr.br,
		bw: mr.bw,
		bb: mr.bb,
	}
	if br, ok := rd.(*prefix.Reader); ok {
		// Use input Reader directly as a prefix.Reader.
		mr.rd = br
	} else {
		// Use pre-allocated prefix.Reader to wrap input Reader.
		mr.rd = &mr.br
		mr.rd.Init(rd, false)
	}
	return
}

// Read reads the decoded meta data from the underlying io.Reader.
// This returns io.EOF either when a meta block with final bits set is found or
// when io.EOF is hit in the underlying reader.
func (mr *Reader) Read(buf []byte) (int, error) {
	if mr.err != nil {
		return 0, mr.err
	}

	var rdCnt int
	for len(buf) > 0 {
		if len(mr.buf) > 0 {
			cpCnt := copy(buf, mr.buf)
			buf, mr.buf = buf[cpCnt:], mr.buf[cpCnt:]
			rdCnt += cpCnt
			break
		}

		if mr.final != FinalNil {
			mr.FinalMode = mr.final
			mr.err = io.EOF
			break
		}

		mr.err = mr.decodeBlock()
		if mr.err != nil {
			break
		}
	}

	mr.OutputOffset += int64(rdCnt)
	return rdCnt, mr.err
}

// Close ends the meta stream.
// The FinalMode encountered becomes valid after calling Close.
func (mr *Reader) Close() error {
	if mr.err == errClosed {
		return nil
	}
	if mr.err != nil && mr.err != io.EOF {
		return mr.err
	}

	mr.FinalMode = mr.final
	mr.err = errClosed
	mr.rd = nil // Release reference to underlying Reader
	return nil
}

// decodeBlock decodes a single meta block from the underlying Reader
// into mr.buf and sets mr.final based on the block's final bits.
// It also manages the statistic variables: InputOffset and NumBlocks.
func (mr *Reader) decodeBlock() (err error) {
	defer errRecover(&err)

	// Update the number of bytes read from underlying Reader.
	offset := mr.rd.Offset
	defer func() {
		if _, errFl := mr.rd.Flush(); errFl != nil {
			err = errFl
		}
		mr.InputOffset += mr.rd.Offset - offset
	}()

	mr.bb.Reset()
	mr.bw.Init(&mr.bb, false)

	var magicBuf [4]byte
	if err := mr.rd.PullBits(1); err != nil {
		if err == io.ErrUnexpectedEOF {
			panic(io.EOF) // EOF is okay for first bit
		}
		panic(err)
	}
	magic := mr.rd.ReadBits(32)
	binary.LittleEndian.PutUint32(magicBuf[:], uint32(magic))
	if ReverseSearch(magicBuf[:]) != 0 {
		panic(ErrCorrupt) // Magic must appear
	}
	finalStream := (magic>>0)&1 > 0
	pads := (magic >> 3) & 7       // 0..7
	numHCLen := 4 + (magic>>13)&15 // 6..18, always even
	if numHCLen < 6 {
		panic(ErrCorrupt)
	}
	for i := uint(5); i < numHCLen-1; i++ {
		if mr.rd.ReadBits(3) != 0 {
			panic(ErrCorrupt) // Empty HCLen code
		}
	}
	if mr.rd.ReadBits(3) != 2 {
		panic(ErrCorrupt) // Final HCLen code
	}
	huffLen := 8 - (numHCLen-4)/2 // Based on XFLATE specification
	huffRange := 1 << uint(huffLen)

	// Read symbols.
	var bit, ones uint
	fifo := byte(0xff)
	for idx := 0; idx < maxSyms; {
		cnt := 1
		sym, ok := mr.rd.TryReadSymbol(&decHuff)
		if !ok {
			sym = mr.rd.ReadSymbol(&decHuff)
		}
		switch sym {
		case symZero:
			bit = 0
			fifo = (fifo >> 1) | byte(0<<7)
		case symOne:
			bit = 1
			fifo = (fifo >> 2) | byte(1<<6)
		case symRepLast:
			val, ok := mr.rd.TryReadBits(2)
			if !ok {
				val = mr.rd.ReadBits(2)
			}
			cnt = int(val + minRepLast)
			fifo = (fifo >> 3) | byte(3<<5)
			fifo = (fifo >> 2) | byte(val<<6)
		case symRepZero:
			bit = 0
			val, ok := mr.rd.TryReadBits(7)
			if !ok {
				val = mr.rd.ReadBits(7)
			}
			cnt = int(val + minRepZero)
			fifo = (fifo >> 3) | byte(7<<5)
			fifo = (fifo >> 7) | byte(val<<1)
		}

		if idx == 0 && sym != symZero {
			panic(ErrCorrupt) // First symbol always symZero
		}
		if fifo == 0x00 {
			// The specification forbids a sequence of 8 zero bits to appear
			// in the symbol section. This ensures that the magic value never
			// appears in the meta encoding by accident.
			panic(ErrCorrupt)
		}
		for i := 0; i < cnt; i++ {
			if ok := mr.bw.TryWriteBits(bit, 1); !ok {
				mr.bw.WriteBits(bit, 1)
			}
			ones += bit
		}
		idx += cnt
	}
	if mr.bw.BitsWritten() != maxSyms {
		panic(ErrCorrupt)
	}
	mr.bw.WriteBits(0, numPads(maxSyms)) // Flush to byte boundary

	// Decode data segment.
	mr.bw.Flush()
	syms := mr.bb.Bytes() // Exactly 33 bytes
	if int(ones) != huffRange {
		panic(ErrCorrupt) // Ensure complete HLitTree
	}
	if i := uint(maxSyms - 1); syms[i/8]&(1<<(i%8)) == 0 {
		panic(ErrCorrupt) // EOM symbol must be set
	}

	flags := syms[0]
	finalMeta := (flags>>1)&1 > 0
	invert := (flags>>2)&1 > 0
	size := (flags >> 3) & 31 // 0..31

	buf := syms[1 : 1+size] // Skip first header byte
	if invert {
		for i, b := range buf {
			buf[i] = ^b
		}
	}

	final := FinalMode(btoi(finalMeta) + btoi(finalStream))
	if finalStream && !finalMeta {
		panic(ErrCorrupt)
	}

	// Decode footer.
	if mr.rd.ReadBits(pads) > 0 {
		panic(ErrCorrupt) // Pads must be zero
	}
	if mr.rd.ReadBits(1) > 0 {
		panic(ErrCorrupt) // HDistTree must be empty
	}
	if mr.rd.ReadBits(huffLen) != uint(huffRange-1) {
		panic(ErrCorrupt) // EOM marker
	}
	if mr.rd.BitsRead()%8 > 0 {
		panic(ErrCorrupt) // Bit reader not byte-aligned
	}

	mr.buf, mr.final = buf, final
	mr.NumBlocks++
	return nil
}
