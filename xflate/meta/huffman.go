// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import "io"
import "github.com/dsnet/golib/bits"

// The Huffman encoding used by the XFLATE meta encoding uses a partially
// pre-determined HCLEN table. The symbols are 0, 16, 18, and another symbol
// between minHuffLen and maxHuffLen, inclusively. The 0 symbol represents a
// logical zero in the meta encoding, and the symbol between minHuffLen and
// maxHuffLen represents a logical one. Symbols 16 and 18 are used to provide a
// primitive form of run-length encoding. The codes that these symbols map to
// are fixed and are shown below.
//
//	Code      Symbol
//	0    <=>  0                      (symZero)
//	10   <=>  minHuffLen..maxHuffLen (symOne)
//	110  <=>  16                     (symRepLast)
//	111  <=>  18                     (symRepZero)
//
// The symZero symbol occupies 1 bit since it is the most commonly needed bit,
// while symOne occupies 2 bits. Thus, it is cheaper to encode logical zeros
// than it is to encode logical ones. The invert bit in the meta encoding takes
// advantage of this fact and allows all data bits to be inverted so that the
// number zero bits is greater than the number of one bits.
type symbol int

const (
	symZero    symbol = iota // Symbol to encode a input zero (clen: 0)
	symOne                   // Symbol to encode a input one  (clen: minHuffLen..maxHuffLen)
	symRepLast               // Symbol to repeat last symbol  (clen: 16)
	symRepZero               // Symbol to repeat zero symbol  (clen: 18)
	maxSym
)

// Write the given Huffman symbol for meta blocks.
// This function panics if an error occurs.
func encodeSym(bw bits.BitsWriter, sym symbol) {
	var err error
	switch sym {
	case symZero:
		_, err = bw.WriteBits(0, 1) // Write '0'
	case symOne:
		_, err = bw.WriteBits(1, 2) // Write '10'
	case symRepLast:
		_, err = bw.WriteBits(3, 3) // Write '110'
	case symRepZero:
		_, err = bw.WriteBits(7, 3) // Write '111'
	default:
		panic("invalid huffman symbol") // This should never occur
	}
	if err != nil {
		panic(err)
	}
}

// Read the next Huffman symbol for meta blocks.
// This function panics if an error occurs.
func decodeSym(br bits.BitsReader) symbol {
	var bit bool
	var err error

	if bit, err = br.ReadBit(); err != nil {
		goto fail
	} else if !bit {
		return symZero // Read '0'
	}
	if bit, err = br.ReadBit(); err != nil {
		goto fail
	} else if !bit {
		return symOne // Read '10'
	}
	if bit, err = br.ReadBit(); err != nil {
		goto fail
	} else if !bit {
		return symRepLast // Read '110'
	}
	return symRepZero // Read '111'

fail:
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	panic(err)
}
