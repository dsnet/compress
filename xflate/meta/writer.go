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

type Writer struct {
	InputOffset  int64 // Total number of bytes issued to Write
	OutputOffset int64 // Total number of bytes written to underlying io.Writer
	BlockCount   int64 // Number of blocks encoded

	// LastMode determines which last bits (if any) to set.
	// This must be set prior to a call to Close.
	LastMode LastMode

	wr io.Writer
	bw prefix.Writer // Temporary bit writer
	bb bytes.Buffer  // Buffer for bw to write into

	cnts   []int
	buf0s  int               // Number of 0-bits in buf
	buf1s  int               // Number of 1-bits in buf
	bufCnt int               // Number of bytes in buf
	buf    [MaxRawBytes]byte // Buffer to collect raw bytes to be encoded
	err    error             // Persistent error
}

func NewWriter(wr io.Writer) *Writer {
	mw := new(Writer)
	mw.Reset(wr)
	return mw
}

func (mw *Writer) Reset(wr io.Writer) {
	*mw = Writer{
		wr:   wr,
		bw:   mw.bw,
		bb:   mw.bb,
		cnts: mw.cnts,
	}
	return
}

func (mw *Writer) Write(buf []byte) (int, error) {
	if mw.err != nil {
		return 0, mw.err
	}

	var wrCnt int
	for _, b := range buf {
		zeros, ones := numBits(b)

		// If possible, avoid flushing to maintain high efficiency.
		if ensured := mw.bufCnt <= EnsureRawBytes; ensured {
			goto skipEncode
		}
		if huffLen, _ := mw.computeHuffLen(mw.buf0s+zeros, mw.buf1s+ones); huffLen > 0 {
			goto skipEncode
		}

		mw.err = mw.encodeBlock(LastNil)
		if mw.err != nil {
			break
		}

	skipEncode:
		mw.buf0s += zeros
		mw.buf1s += ones
		mw.buf[mw.bufCnt] = b
		mw.bufCnt++
		wrCnt++
	}

	mw.InputOffset += int64(wrCnt)
	return wrCnt, mw.err
}

// Close ends the meta stream and flushes all buffered data.
// The desired LastMode must be set prior to calling Close.
func (mw *Writer) Close() error {
	if mw.err == errClosed {
		return nil
	}
	if mw.err != nil {
		return mw.err
	}

	err := mw.encodeBlock(mw.LastMode)
	if err != nil {
		mw.err = err
	} else {
		mw.err = errClosed
	}
	mw.wr = nil // Release reference to underlying Writer
	return err
}

// computeHuffLen computes the shortest Huffman length to encode the data.
// If the input data is too large, then 0 is returned.
func (*Writer) computeHuffLen(zeros, ones int) (huffLen uint, inv bool) {
	if inv = ones > zeros; inv {
		zeros, ones = ones, zeros
	}
	for huffLen = minHuffLen; huffLen <= maxHuffLen; huffLen++ {
		maxOnes := 1 << uint(huffLen)
		if maxSyms-maxOnes >= zeros+8 && maxOnes >= ones+8 {
			return huffLen, inv
		}
	}
	return 0, false
}

// computeCounts computes counts of necessary 0s and 1s to form the data.
// A positive count of +n means to repeat a '1' bit n times,
// while a negative count of -n means to repeat a '0' bit n times.
//
// For example (LSB on left):
//	01101011 11100011  =>  [-1, +2, -1, +1, -1, +5, -3, +2]
func (mw *Writer) computeCounts(buf []byte, maxOnes int, last, invert bool) []int {
	// Stack copy of buf for safe mutations.
	var arr [1 + MaxRawBytes]byte
	copy(arr[1:], buf)
	flags := &arr[0]
	buf = arr[1 : 1+len(buf)]
	if invert {
		for i, b := range buf {
			buf[i] = ^b
		}
	}

	// Set the flags.
	*flags |= byte(0) << 0            // Always start with zero bit
	*flags |= byte(btoi(last)) << 1   // Status bit as last meta block
	*flags |= byte(btoi(invert)) << 2 // Status bit that data is inverted
	*flags |= byte(len(buf)) << 3     // Data size

	// Compute the counts.
	var zeros, ones int
	cnts, pcnt := mw.cnts[:0], 0
	for _, b := range arr[:1+len(buf)] {
		for b := int(b) | (1 << 8); b != 1; b >>= 1 { // Data bits (LSB first)
			if (b&1 > 0) != (pcnt > 0) {
				cnts, pcnt = append(cnts, pcnt), 0
			}
			pcnt += (b&1)*2 - 1 // Add +1 or -1
		}
		b0s, b1s := numBits(b)
		zeros, ones = zeros+b0s, ones+b1s
	}
	if pcnt > 0 {
		cnts, pcnt = append(cnts, pcnt), 0
	}
	pcnt += -1 * (maxSyms - maxOnes - zeros) // Add needed zeros
	if pcnt < 0 {
		cnts, pcnt = append(cnts, pcnt), 0
	}
	pcnt += +1 * (maxOnes - ones) // Add needed ones (includes EOM)
	cnts = append(cnts, pcnt)

	mw.cnts = cnts
	return cnts
}

// encodeBlock encodes a single meta block from mw.buf into the
// underlying Writer. The values buf0s and buf1s must accurately reflect
// what is in buf. If successful, it will clear bufCnt, buf0s, and buf1s.
// It also manages the statistic variables: OutputOffset and BlockCount.
func (mw *Writer) encodeBlock(last LastMode) (err error) {
	defer errRecover(&err)

	mw.bb.Reset()
	mw.bw.Init(&mw.bb, false)

	buf := mw.buf[:mw.bufCnt]
	huffLen, inv := mw.computeHuffLen(mw.buf0s, mw.buf1s)
	if huffLen == 0 {
		panic(ErrInvalid)
	}

	// Encode header.
	numHCLen := 4 + (8-huffLen)*2 // Based on XFLATE specification
	magic := uint(binary.LittleEndian.Uint32(magicVals[:]))
	magic |= uint(btoi(last == LastStream)) << 0 // Set last DEFLATE block bit
	magic |= uint(numHCLen-4) << 13              // numHCLen: 6..18, always even
	mw.bw.WriteBits(magic, 32)
	for i := uint(5); i < numHCLen-1; i++ {
		mw.bw.WriteBits(0, 3) // Empty HCLen code
	}
	mw.bw.WriteBits(2, 3) // Final HCLen code

	// Encode data segment.
	cnts := mw.computeCounts(buf, 1<<uint(huffLen), last != LastNil, inv)
	val, pre := 0, 0
	for len(cnts) > 0 {
		if cnts[0] == 0 {
			cnts = cnts[1:]
			continue
		}
		sym := btoi(cnts[0] > 0) // If zero:  0, if one:  1
		cur := sym*2 - 1         // If zero: -1, if one: +1
		cnt := cur * cnts[0]     // Count as positive integer

		switch {
		case pre != 0 && cur < 0 && cnt >= minRepZero: // Use repeated zero code
			if val = maxRepZero; val > cnt {
				val = cnt
			}
			if ok := mw.bw.TryWriteSymbol(symRepZero, &encHuff); !ok {
				mw.bw.WriteSymbol(symRepZero, &encHuff)
			}
			if ok := mw.bw.TryWriteBits(uint(val-minRepZero), 7); !ok {
				mw.bw.WriteBits(uint(val-minRepZero), 7)
			}
		case pre == cur && cnt >= minRepLast: // Use repeated last code
			if val = maxRepLast; val > cnt {
				val = cnt
			}
			if ok := mw.bw.TryWriteSymbol(symRepLast, &encHuff); !ok {
				mw.bw.WriteSymbol(symRepLast, &encHuff)
			}
			if ok := mw.bw.TryWriteBits(uint(val-minRepLast), 2); !ok {
				mw.bw.WriteBits(uint(val-minRepLast), 2)
			}
		default: // Use literal value
			val = 1
			if ok := mw.bw.TryWriteSymbol(uint(sym), &encHuff); !ok {
				mw.bw.WriteSymbol(uint(sym), &encHuff)
			}
		}

		cnts[0] -= cur * val // Decrement count
		pre = cur            // Store previous sign
	}

	// Encode footer (and update header with known padding size).
	pads := numPads(uint(mw.bw.BitsWritten()) + 1 + huffLen)
	mw.bw.WriteBits(0, pads)                 // Pad to nearest byte
	mw.bw.WriteBits(0, 1)                    // Empty HDistTree
	mw.bw.WriteBits((1<<huffLen)-1, huffLen) // Encode EOM marker

	mw.bw.Flush()                       // Flush all data to the bytes.Buffer
	mw.bb.Bytes()[0] |= byte(pads) << 3 // Update NumHLit size

	// Write the encoded block.
	cnt, err := mw.wr.Write(mw.bb.Bytes())
	mw.OutputOffset += int64(cnt)
	if err != nil {
		panic(err)
	}
	mw.bufCnt, mw.buf0s, mw.buf1s = 0, 0, 0
	mw.BlockCount++
	return nil
}
