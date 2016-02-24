// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import "io"
import "github.com/dsnet/golib/bits"
import "github.com/dsnet/golib/errs"

type Writer struct {
	wr     io.Writer         // Underlying writer
	cnt    int64             // Total number of bytes written
	blkCnt int64             // Total number of blocks written
	buf0s  int               // Number of 0-bits in buf
	buf1s  int               // Number of 1-bits in buf
	bufCnt int               // Number of bytes in buf
	buf    [MaxRawBytes]byte // Buffer to collect raw bytes to be encoded
	last   LastMode          // Last bits to be set upon Close
	err    error             // Persistent error

	// These fields are lazily allocated and reused for efficiency.
	bb   bits.Buffer
	cnts []int
}

// NewWriter creates a new Writer.
//
// The last mode determines which last bits should be set in the last block when
// the Close method is called. If the output of multiple writers are going to be
// concatenated together, then LastNil or LastMeta should be used. If this
// writer should mark the end of a DEFLATE stream, then use LastStream.
func NewWriter(wr io.Writer, last LastMode) *Writer {
	mw := new(Writer)
	mw.Reset(wr, last)
	return mw
}

// WriteCount reports the number of bytes written to the underlying writer.
func (mw *Writer) WriteCount() int64 { return mw.cnt }

// BlockCount reports the number of blocks successfully written.
func (mw *Writer) BlockCount() int64 { return mw.blkCnt }

// Write writes multiple bytes, flushing only when necessary.
// If no bytes have been written thus far, this is guaranteed to write at least
// EnsureRawBytes in a single meta block.
func (mw *Writer) Write(buf []byte) (cnt int, err error) {
	for idx, val := range buf {
		if err = mw.WriteByte(val); err != nil {
			return idx, err
		}
	}
	return len(buf), nil
}

// WriteByte writes the next byte. The Writer will buffer as many bytes as will
// fit in a single meta block before flushing.
func (mw *Writer) WriteByte(val byte) (err error) {
	if err == nil {
		ones := bits.CountByte(val)
		zeros := 8 - ones
		notEnsured := mw.bufCnt >= EnsureRawBytes
		if notEnsured && mw.computeHuffLen(mw.buf0s+zeros, mw.buf1s+ones) == 0 {
			err = mw.flush(LastNil)
		}
		if err == nil {
			mw.buf0s += zeros
			mw.buf1s += ones
			mw.buf[mw.bufCnt] = val
			mw.bufCnt++
		}
	}
	return err
}

// Close closes the writer.
// It will flush the last block with appropriate last bits set.
func (mw *Writer) Close() error {
	err := mw.flush(mw.last)
	if err == nil {
		mw.err = io.ErrClosedPipe
	}
	return err
}

// Reset resets the Writer with a new io.Writer.
func (mw *Writer) Reset(wr io.Writer, last LastMode) {
	mw.wr, mw.cnt, mw.blkCnt = wr, 0, 0
	mw.bufCnt, mw.buf0s, mw.buf1s = 0, 0, 0
	mw.last, mw.err = last, nil

	mw.bb.Reset()
	mw.cnts = mw.cnts[:0]
}

// Flush the current buffer. This encodes the current buffer as a single meta
// block. The invariants maintained by ReadByte and Read ensure that the buffer
// is encodable in a single block.
func (mw *Writer) flush(last LastMode) error {
	var wrCnt int
	if mw.err == nil {
		buf := mw.buf[:mw.bufCnt]
		wrCnt, mw.err = mw.encodeBlock(mw.wr, buf, last)
		if mw.err == nil {
			mw.bufCnt, mw.buf0s, mw.buf1s = 0, 0, 0
			mw.blkCnt++
		}
		mw.cnt += int64(wrCnt)
	}
	return mw.err
}

// Compute the shortest Huffman length needed to encode the data.
// If we fail to compute a valid huffLen, then the input data is too large.
//
// This function does not depend on Writer, but is related to work it does.
func (*Writer) computeHuffLen(zeros, ones int) int {
	if ones > zeros { // If too many ones, invert the data
		zeros, ones = ones, zeros
	}
	for huffLen := minHuffLen; huffLen <= maxHuffLen; huffLen++ {
		maxOnes := 1 << uint(huffLen)
		if maxSyms-maxOnes >= zeros+8 && maxOnes >= ones+8 {
			return huffLen
		}
	}
	return 0
}

// Compute the counts of necessary 0s and 1s to form the data. A positive count
// of +n means to repeat a '1' bit n times, while a negative count of -n means
// to repeat a '0' bit n times.
//
// For example (LSB on left):
//	01101011 11100011  =>  [-1, +2, -1, +1, -1, +5, -3, +2]
//
// The only state that this function depends on in mw is mw.cnts.
// It reuses this object for efficiency purposes.
func (mw *Writer) computeCounts(data []byte, maxOnes int, invert, last bool) []int {
	zeros, ones := 0, 0
	mw.cnts = append(mw.cnts[:0], 0)
	addCnts := func(n int) {
		if (n > 0) != (mw.cnts[len(mw.cnts)-1] > 0) {
			mw.cnts = append(mw.cnts, 0) // Polarity changed, so add new slot
		}
		mw.cnts[len(mw.cnts)-1] += n // Increment count
		zeros -= min(0, n)
		ones += max(0, n)
	}

	addCnts(-1)                                         // Always start with zero
	addCnts(sign(last))                                 // Status bit as last meta block
	addCnts(sign(invert))                               // Status bit that data is inverted
	for sz := len(data) | (1 << 5); sz != 1; sz >>= 1 { // Data size (LSB first)
		addCnts(sign(sz&1 > 0))
	}

	// The code below is an optimized form of the following:
	/*
		for _, val := range data {
			for val := int(val) | (1 << 8); val != 1; val >>= 1 { // Data bits (LSB first)
				addCnts(sign(invert != (val&1 > 0)))
			}
		}
	*/
	inv := byte(0x00)
	dataOnes := bits.Count(data)
	if invert {
		inv = byte(0xff) // XOR with 0xff is effectively invert
		dataOnes = 8*len(data) - dataOnes
	}
	pcnt := &mw.cnts[len(mw.cnts)-1] // Pointer to last count
	for _, val := range data {
		for val := int(val^inv) | (1 << 8); val != 1; val >>= 1 { // Data bits (LSB first)
			if (val&1 > 0) != (*pcnt > 0) {
				mw.cnts = append(mw.cnts, 0) // Polarity changed, so add new slot
				pcnt = &mw.cnts[len(mw.cnts)-1]
			}
			*pcnt += (val&1)*2 - 1 // Add +1 or -1; same as sign(val&1 > 0)
		}
	}
	ones += dataOnes
	zeros += 8*len(data) - dataOnes

	addCnts(-1 * (maxSyms - maxOnes - zeros)) // Add needed zeros
	addCnts(+1 * (maxOnes - ones))            // Add needed ones (includes EOM)
	return mw.cnts
}

// Encode the input data into a single meta block.
// The count returned is the number of bytes written to wr.
//
// The only state that this function depends on in mw is mw.bb and mw.cnts.
// It reuses these objects for efficiency purposes.
func (mw *Writer) encodeBlock(wr io.Writer, data []byte, last LastMode) (cnt int, err error) {
	defer errs.Recover(&err)

	ones := bits.Count(data)
	zeros := 8*len(data) - ones
	huffLen := mw.computeHuffLen(zeros, ones)
	errs.Assert(huffLen > 0, errMetaInvalid)

	bb := &mw.bb
	bb.Reset()

	// Encode header.
	numHCLen := 4 + (8-huffLen)*2 // Based on XFLATE specification
	bb.Write(magicVals[:])
	bits.Set(bb.Bytes(), last == LastStream, 0)
	bits.SetN(bb.Bytes(), uint(numHCLen-4), 4, 13) // 6..18, always even
	for idx := 5; idx < numHCLen-1; idx++ {
		bb.WriteBits(0, 3) // Empty HCLen code
	}
	bb.WriteBits(2, 3) // Final HCLen code

	// Encode data segment.
	cnts := mw.computeCounts(data, 1<<uint(huffLen), ones > zeros, last != LastNil)
	val, pre := 0, 0
	for len(cnts) > 0 {
		cur := sign(cnts[0] > 0) // If zero: -1, if one: +1
		sym := (cur + 1) / 2     // If zero:  0, if one:  1
		ext := 1 - cur           // If zero:  2, if one:  0
		cnt := cur * cnts[0]     // Count as positive integer

		// The ext variable is 2 if we are encoding zero bits. The use of this
		// variable is to restrict when we can use symRepLast. The reason is
		// that symRepLast (and its additional 2bit count) occupies a total of
		// 5 bits, while each zero bit occupies 1bit. Thus, it is more efficient
		// to encode using symRepLast only if there are 5 or more zero bits.

		switch {
		case cur < 0 && cnt >= minRepZero: // Use repeated zero code
			val = min(maxRepZero, cnt)
			encodeSym(bb, symRepZero)
			bb.WriteBits(uint(val-minRepZero), 7)
		case pre == cur && cnt >= minRepLast+ext: // Use repeated last code
			val = min(maxRepLast, cnt)
			encodeSym(bb, symRepLast)
			bb.WriteBits(uint(val-minRepLast), 2)
		case cnt > 0: // Use literal value
			val = 1
			encodeSym(bb, symbol(sym))
		default: // Discard count if empty
			cnts = cnts[1:]
			continue
		}

		cnts[0] -= cur * val // Decrement count
		pre = cur            // Store previous sign
	}

	// Encode footer.
	pads := numPads(int(bb.BitsWritten()) + 1 + huffLen)
	bb.WriteBits(0, pads)                       // Pad to nearest byte
	bb.WriteBits(0, 1)                          // Empty HDistTree
	bb.WriteBits((1<<uint(huffLen))-1, huffLen) // Encode EOM marker
	bits.SetN(bb.Bytes(), uint(pads), 3, 3)     // Update NumHLit size

	errs.Assert(bb.WriteAligned(), errMetaInvalid) // This should never occur
	return wr.Write(bb.Bytes())                    // Final write deals with IO errors
}
