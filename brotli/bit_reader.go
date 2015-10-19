// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"
import "bufio"

// TODO(dsnet): If we compute the minimum number of bits we can safely read, is
// it large enough that we can just use an io.Reader alone without performance
// detriments? It would be nice to avoid using io.ByteReader.
type byteReader interface {
	io.Reader
	io.ByteReader
}

type bitReader struct {
	rd io.Reader
	rb io.ByteReader

	offset  int64 // Number of bytes read from the underlying reader
	bufBits uint32
	numBits uint
}

func (br *bitReader) Reset(r io.Reader) {
	if rr, ok := r.(byteReader); ok {
		*br = bitReader{rd: rr, rb: rr}
	} else {
		rr = bufio.NewReader(r)
		*br = bitReader{rd: rr, rb: rr}
	}
}

// ReadFull reads len(buf) bytes into buf.
// If an IO error occurs, then it panics.
func (br *bitReader) ReadFull(buf []byte) {
	if br.numBits%8 > 0 {
		panic(Error("brotli: unaligned byte read"))
	}

	for len(buf) > 0 {
		if br.numBits > 0 {
			buf[0] = byte(br.numBits)
			buf = buf[1:]
		} else {
			cnt, err := io.ReadFull(br.rd, buf)
			buf = buf[cnt:]
			br.offset += int64(cnt)
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				panic(err)
			}
		}
	}
	return
}

// ReadBits reads nb bits in LSB order from the underlying reader.
// If an IO error occurs, then it panics.
func (br *bitReader) ReadBits(nb uint) uint {
	for br.numBits < nb {
		c, err := br.rb.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			panic(err)
		}
		br.offset++
		br.bufBits |= uint32(c) << br.numBits
		br.numBits += 8
	}
	val := uint(br.bufBits & uint32(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}

// ReadPads reads 0-7 bits from the underlying reader to achieve byte-alignment.
func (br *bitReader) ReadPads() uint {
	nb := br.numBits % 8
	val := uint(br.bufBits & uint32(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}
