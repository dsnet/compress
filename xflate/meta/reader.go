// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import "io"
import "github.com/dsnet/golib/ioutil"
import "github.com/dsnet/golib/bits"
import "github.com/dsnet/golib/errs"

// The actual read interface needed by NewReader.
type byteReader interface {
	io.Reader
	io.ByteReader
}

type Reader struct {
	rd     byteReader // Underlying reader
	cnt    int64      // Total number of bytes read
	blkCnt int64      // Total number of blocks read
	buf    []byte     // Decoded data yet to be consumed
	last   LastMode   // Last bits set on latest block read
	err    error      // Persistent error

	// These fields are lazily allocated and reused for efficiency.
	brd ioutil.ByteReader
	tee ioutil.TeeByteReader
	blk bits.Buffer
	bb  bits.Buffer
	br  bits.Reader
}

// NewReader creates a new Reader.
func NewReader(rd io.Reader) *Reader {
	mr := new(Reader)
	mr.Reset(rd)
	return mr
}

// ReadCount reports the number of bytes read from the underlying reader.
func (mr *Reader) ReadCount() int64 { return mr.cnt }

// BlockCount reports the number of blocks successfully read.
func (mr *Reader) BlockCount() int64 { return mr.blkCnt }

// LastMarker reports which last bits were set in the last block read.
func (mr *Reader) LastMarker() LastMode { return mr.last }

// BlockData provides the encoded data of the last block read. If there was an
// error decoding the last block, then the malformed block can be obtained here.
func (mr *Reader) BlockData() []byte { return mr.blk.Bytes() }

// AtEOF reports whether Reader is currently at io.EOF.
func (mr *Reader) AtEOF() bool {
	_, err := mr.PeekByte()
	return err == io.EOF
}

// Read reads multiple bytes.
func (mr *Reader) Read(data []byte) (cnt int, err error) {
	for len(data) > 0 && err == nil {
		err = mr.fetch()
		cpCnt := copy(data, mr.buf)
		data, mr.buf = data[cpCnt:], mr.buf[cpCnt:]
		cnt += cpCnt
	}
	return cnt, err
}

// ReadByte reads the next byte. If necessary, this will fetch data from the
// next block and buffer it. It will always fetch the next block unless it hits
// io.EOF, detects the last meta block bit, or detects the last stream bit.
// The LastMarker method determines which ending condition was hit.
func (mr *Reader) ReadByte() (val byte, err error) {
	err = mr.fetch()
	if len(mr.buf) > 0 {
		val, mr.buf = mr.buf[0], mr.buf[1:]
		return val, nil
	}
	return 0, err
}

// PeekByte reads the next byte without advancing the read pointer.
// Obviously, this will cause the next meta block to be buffered if necessary.
func (mr *Reader) PeekByte() (val byte, err error) {
	err = mr.fetch()
	if len(mr.buf) > 0 {
		return mr.buf[0], nil
	}
	return 0, err
}

// Reset resets the Reader with a new io.Reader.
func (mr *Reader) Reset(rd io.Reader) {
	// For efficiency, rd should satisfy the io.ByteReader interface as well.
	// Otherwise, it will wrap the input with a single byte buffer reader.
	brd, ok := rd.(byteReader)
	if !ok {
		mr.brd.Reader = rd
		brd = &mr.brd
	}

	mr.rd, mr.cnt, mr.blkCnt = brd, 0, 0
	mr.last, mr.buf, mr.err = LastNil, nil, nil

	mr.tee = ioutil.TeeByteReader{R: mr.rd, W: &mr.blk}
	mr.blk.Reset()
	mr.bb.Reset()
	mr.br.Reset(nil)
}

// Fetch the next block's data. If the current block is empty, it will keep
// fetching more blocks until it retrieves a non-empty one. Fetch only triggers
// if the current buffer is empty and no read errors were encountered thus far.
func (mr *Reader) fetch() error {
	if len(mr.buf) > 0 {
		return nil
	}

	var rdCnt int
	for len(mr.buf) == 0 && mr.err == nil {
		// The previous block had last bits set, so stop.
		if mr.last != LastNil {
			mr.err = io.EOF
			break
		}

		// Read the block.
		mr.blk.Reset() // Clear out previous copy of the block
		mr.buf, mr.last, rdCnt, mr.err = mr.decodeBlock(&mr.tee)
		if ReverseSearch(mr.blk.Bytes()) > 0 {
			mr.err = errMetaCorrupt // Magic value found in middle of block
		} else if mr.err == nil {
			mr.blkCnt++
		}
		mr.cnt += int64(rdCnt)
	}
	return mr.err
}

// Decode a single meta block.
// The count returned is the number of bytes read from rd.
// This returns io.EOF if and only if no bytes are read at all.
//
// The only state that this function depends on in mr is mr.bb and mr.br.
// It reuses these objects for efficiency purposes.
func (mr *Reader) decodeBlock(rd io.ByteReader) (data []byte, last LastMode, cnt int, err error) {
	defer errs.Recover(&err)

	bb, br := &mr.bb, &mr.br
	bb.Reset()
	br.Reset(rd)
	defer func() { cnt = int(br.BytesRead()) }()

	// Read magic header.
	n, err := ioutil.ByteCopyN(bb, br, magicLen)
	if n > 0 && err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	errs.Panic(err)

	// Decode header.
	errs.Assert(ReverseSearch(bb.Bytes()) == 0, errMetaCorrupt) // Magic must appear
	lastStream := bits.Get(bb.Bytes(), 0)
	pads := int(bits.GetN(bb.Bytes(), 3, 3))          // 0..7
	numHCLen := int(bits.GetN(bb.Bytes(), 4, 13)) + 4 // 6..18, even only
	errs.Assert(numHCLen >= 6, errMetaCorrupt)        // Magic mask guarantees that numHCLen is even
	for idx := 5; idx < numHCLen-1; idx++ {
		errs.Assert(readBits(br, 3) == 0, errMetaCorrupt) // Empty HCLen code
	}
	errs.Assert(readBits(br, 3) == 2, errMetaCorrupt) // Final HCLen code

	huffLen := 8 - (numHCLen-4)/2 // Based on XFLATE specification
	huffRange := 1 << uint(huffLen)

	// Read symbols.
	bb.Reset()
	ones := 0
	var bit bool
	for idx := 0; idx < maxSyms; {
		cnt := 1
		switch decodeSym(br) {
		case symZero:
			bit = false
		case symOne:
			bit = true
		case symRepLast:
			errs.Assert(idx > 0, errMetaCorrupt)
			cnt = int(readBits(br, 2) + minRepLast)
		case symRepZero:
			bit = false
			cnt = int(readBits(br, 7) + minRepZero)
		}
		bits.WriteSameBit(bb, bit, cnt)
		if bit { // Keep running count of total ones
			ones += cnt
		}
		idx += cnt
	}
	errs.Assert(bb.BitsWritten() == maxSyms, errMetaCorrupt)
	bb.WriteBits(0, numPads(maxSyms)) // Flush to byte boundary

	// Decode data segment.
	syms := bb.Bytes()                                     // Exactly 33 bytes
	errs.Assert(ones == huffRange, errMetaCorrupt)         // Ensure complete HLitTree
	errs.Assert(!bits.Get(syms, 0), errMetaCorrupt)        // First symbol always symZero
	errs.Assert(bits.Get(syms, maxSyms-1), errMetaCorrupt) // EOM symbol set
	lastMeta := bits.Get(syms, 1)
	invert := bits.Get(syms, 2)
	size := bits.GetN(syms, 5, 3) // 0..31

	data = syms[1 : 1+size] // Skip first header byte
	if invert {
		bits.Invert(data)
	}
	last = LastMode(bits.Btoi(lastMeta) + bits.Btoi(lastStream))
	errs.Assert(!lastStream || lastMeta, errMetaCorrupt)

	// Decode footer.
	errs.Assert(readBits(br, pads) == 0, errMetaCorrupt)                    // Pads must be zero
	errs.Assert(readBits(br, 1) == 0, errMetaCorrupt)                       // HDistTree
	errs.Assert(readBits(br, huffLen) == uint(huffRange-1), errMetaCorrupt) // EOM marker
	errs.Assert(br.ReadAligned(), errMetaCorrupt)
	return data, last, 0, nil
}
