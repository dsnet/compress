// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"
import "bufio"

// TODO(dsnet): Most of this logic is identical to compress/flate.
// Centralize common logic to compress/internal/prefix.

type byteReader interface {
	io.Reader
	io.ByteReader
}

type bitReader struct {
	rd      byteReader
	prefix  prefixDecoder // Local prefix decoder
	bufBits uint32        // Buffer to hold some bits
	numBits uint          // Number of valid bits in bufBits
	offset  int64         // Number of bytes read from the underlying reader
}

func (br *bitReader) Init(r io.Reader) {
	if rr, ok := r.(byteReader); ok {
		*br = bitReader{rd: rr}
	} else {
		*br = bitReader{rd: bufio.NewReader(r)}
	}
}

// Read reads up to len(buf) bytes into buf.
func (br *bitReader) Read(buf []byte) (int, error) {
	if br.numBits%8 > 0 {
		return 0, Error("unaligned byte read")
	}

	bufBase := buf
	for len(buf) > 0 {
		if br.numBits > 0 {
			buf[0] = byte(br.bufBits)
			br.bufBits >>= 8
			br.numBits -= 8
			buf = buf[1:]
		} else {
			cnt, err := br.rd.Read(buf)
			buf = buf[cnt:]
			br.offset += int64(cnt)
			if err != nil {
				return len(bufBase) - len(buf), err
			}
		}
	}
	return len(bufBase) - len(buf), nil
}

// ReadBits reads nb bits in LSB order from the underlying reader.
// If an IO error occurs, then it panics.
func (br *bitReader) ReadBits(nb uint) uint {
	for br.numBits < nb {
		c, err := br.rd.ReadByte()
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

// ReadPads reads 0-7 bits from the bit buffer to achieve byte-alignment.
func (br *bitReader) ReadPads() uint {
	nb := br.numBits % 8
	val := uint(br.bufBits & uint32(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}

// ReadSymbol reads the next prefix symbol using the provided prefixDecoder.
// If an IO error occurs, then it panics.
func (br *bitReader) ReadSymbol(pd *prefixDecoder) uint {
	if len(pd.chunks) == 0 {
		panic(ErrCorrupt) // Decode with empty tree
	}

	nb := uint(pd.minBits)
	for {
		for br.numBits < nb {
			// This section is an inlined version of the inner loop of ReadBits
			// for performance reasons.
			c, err := br.rd.ReadByte()
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
		chunk := pd.chunks[uint16(br.bufBits)&pd.chunkMask]
		nb = uint(chunk & prefixCountMask)
		if nb > uint(pd.chunkBits) {
			linkIdx := chunk >> prefixCountBits
			chunk = pd.links[linkIdx][uint16(br.bufBits>>pd.chunkBits)&pd.linkMask]
			nb = uint(chunk & prefixCountMask)
		}
		if nb <= br.numBits {
			br.bufBits >>= nb
			br.numBits -= nb
			return uint(chunk >> prefixCountBits)
		}
	}
}
