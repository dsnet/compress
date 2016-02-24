// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package meta implements the XFLATE meta encoding scheme.
//
// The XFLATE meta encoding is a method of encoding arbitrary data into one
// or more RFC 1951 compliant DEFLATE blocks. This encoding has the special
// property that when the blocks are decoded by a RFC 1951 compliant
// decompressor, they produce absolutely no output. However, when decoded with
// the XFLATE meta decoder, it losslessly produces the encoded input.
//
// The meta encoding works by encoding arbitrary data into the Huffman tree
// definition of dynamic DEFLATE blocks. These blocks have an empty data section
// and produce no output. Due to the Huffman definition overhead, the encoded
// output is usually larger than the input. However, when the input size is
// large, this encoding scheme is able to achieve an efficiency of at least 50%.
//
// Although not designed for this purpose, the meta encoder can be used in
// steganography applications by hiding arbitrary data into any DEFLATE stream.
// One should note however, that the encoding has a very distinctive pattern
// that makes it really easy to detect.
package meta

import (
	"runtime"

	"github.com/dsnet/compress/internal/prefix"
)

// These are the magic values that begin every single meta block.
const magicLen = 4

var magicVals = [magicLen]byte{0x04, 0x00, 0x86, 0x05}
var magicMask = [magicLen]byte{0xc6, 0x3f, 0xfe, 0xff}

const (
	maxSyms    = 257 // Maximum number of literal codes (with EOM marker)
	minHuffLen = 1   // Minimum number of bits for each huffman code
	maxHuffLen = 7   // Maximum number of bits for each huffman code
	minRepLast = 3   // Minimum number of repeated codes (clen: 16)
	maxRepLast = 6   // Maximum number of repeated codes (clen: 16)
	minRepZero = 11  // Minimum number of repeated zeros (clen: 18)
	maxRepZero = 138 // Maximum number of repeated zeros (clen: 18)

	MinRawBytes    = 0  // Theoretical minimum number of bytes a single meta block can encode
	MaxRawBytes    = 31 // Theoretical maximum number of bytes a single meta block can encode
	MinEncBytes    = 12 // Theoretical minimum number of bytes a single meta block will occupy
	MaxEncBytes    = 64 // Theoretical maximum number of bytes a single meta block will occupy
	EnsureRawBytes = 22 // Number of bytes that a single block is ensured to encode (computed using brute force)
)

// LastMode indicates which last bits are set in a meta stream.
type LastMode int

const (
	LastNil    LastMode = iota // Not last meta block, not last DEFLATE stream block
	LastMeta                   // Last meta block, but still not last DEFLATE stream block
	LastStream                 // Last meta block, and last DEFLATE stream block
)

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
const (
	symZero    = iota // Symbol to encode a input zero (clen: 0)
	symOne            // Symbol to encode a input one  (clen: minHuffLen..maxHuffLen)
	symRepLast        // Symbol to repeat last symbol  (clen: 16)
	symRepZero        // Symbol to repeat zero symbol  (clen: 18)
)

var (
	oneBits [256]byte

	encHuff prefix.Encoder
	decHuff prefix.Decoder
)

func init() {
	// Dynamically generate the LUT to count bits.
	for i := range oneBits[:] {
		for b := byte(i); b > 0; b >>= 1 {
			oneBits[i] += b & 1
		}
	}

	// Dynamically generate the Huffman codes.
	var codes = [4]prefix.PrefixCode{
		{Sym: symZero, Len: 1},
		{Sym: symOne, Len: 2},
		{Sym: symRepLast, Len: 3},
		{Sym: symRepZero, Len: 3},
	}
	prefix.GeneratePrefixes(codes[:])
	decHuff.Init(codes[:])
	encHuff.Init(codes[:])
}

// Error is the wrapper type for all errors specific to this package.
type Error string

func (e Error) Error() string  { return "meta: " + string(e) }
func (e Error) CompressError() {}

var (
	errClosed  error = Error("stream is closed")
	ErrInvalid error = Error("cannot encode block")
	ErrCorrupt error = Error("block is corrupted")
)

func errRecover(err *error) {
	switch ex := recover().(type) {
	case nil:
		// Do nothing.
	case runtime.Error:
		panic(ex)
	case error:
		*err = ex
	default:
		panic(ex)
	}
}

// ReverseSearch searches for a meta header in reverse. This returns the last
// index where the header was found. If not found, it returns -1.
func ReverseSearch(data []byte) int {
revLoop:
	for i := len(data) - magicLen; i >= 0; i-- {
		for i, v := range data[i : i+magicLen] {
			if v&magicMask[i] != magicVals[i] {
				continue revLoop
			}
		}
		return i
	}
	return -1
}

// numBits counts the number of zero and one bits in the byte.
func numBits(b byte) (zeros, ones int) {
	ones = int(oneBits[b])
	zeros = 8 - ones
	return
}

// numPads computes number of bits needed to pad n-bits to a byte alignment.
func numPads(n uint) uint {
	return -n & 7
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
