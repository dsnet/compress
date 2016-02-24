// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package meta implements the XFLATE meta encoding scheme.
//
// The XFLATE meta encoding is a method of encoding arbitrary data into one
// or more RFC1951 compliant DEFLATE blocks. This encoding has the special
// property that when the blocks are decoded by a RFC1951 compliant
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

type LastMode int

const (
	LastNil    LastMode = iota // Not last meta block, not last DEFLATE stream block
	LastMeta                   // Last meta block, but still not last DEFLATE stream block
	LastStream                 // Last meta block, and last DEFLATE stream block
)

// Error is the wrapper type for all errors returned by this library.
type Error string

func (e Error) Error() string { return string(e) }

var (
	errMetaInvalid error = Error("xflate/meta: cannot encode data into meta block")
	errMetaCorrupt error = Error("xflate/meta: meta block format is corrupted")
)

// ReverseSearch searches for a meta header. This returns the index where the
// header was found. If not found, it returns -1.
func ReverseSearch(data []byte) (pos int) {
RevLoop:
	for idx := len(data) - len(magicVals); idx >= 0; idx-- {
		for idx, val := range data[idx : idx+len(magicVals)] {
			if val&magicMask[idx] != magicVals[idx] {
				continue RevLoop
			}
		}
		return idx
	}
	return -1
}
