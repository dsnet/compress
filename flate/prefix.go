// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

const maxPrefixBits = 15

const (
	maxNumCLenSyms = 19
	maxNumLitSyms  = 286
	maxNumDistSyms = 30
)

var (
	lenLUT   [maxNumLitSyms - 257]rangeCode // RFC section 3.2.5
	distLUT  [maxNumDistSyms]rangeCode      // RFC section 3.2.5
	litTree  prefixDecoder                  // RFC section 3.2.6
	distTree prefixDecoder                  // RFC section 3.2.6
)

type rangeCode struct {
	base uint32 // Starting base offset of the range
	bits uint32 // Bit-width of a subsequent integer to add to base offset
}

type prefixCode struct {
	sym uint32 // The symbol being mapped
	val uint32 // Value of the prefix code (must be in [0..1<<len])
	len uint32 // Bit length of the prefix code
}

var (
	// RFC section 3.2.7.
	// Prefix code lengths for code lengths alphabet.
	clenLens = [maxNumCLenSyms]uint{
		16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15,
	}
)

func initPrefixLUTs() {
	// These come from the RFC section 3.2.5.
	for i, base := 0, 3; i < len(lenLUT)-1; i++ {
		nb := uint(i/4 - 1)
		if i < 4 {
			nb = 0
		}
		lenLUT[i] = rangeCode{base: uint32(base), bits: uint32(nb)}
		base += 1 << nb
	}
	lenLUT[len(lenLUT)-1] = rangeCode{base: 258, bits: 0}

	// These come from the RFC section 3.2.5.
	for i, base := 0, 1; i < len(distLUT); i++ {
		nb := uint(i/2 - 1)
		if i < 2 {
			nb = 0
		}
		distLUT[i] = rangeCode{base: uint32(base), bits: uint32(nb)}
		base += 1 << nb
	}

	// These come from the RFC section 3.2.6.
	var litCodes [288]prefixCode
	for i := 0; i < 144; i++ {
		litCodes[i] = prefixCode{sym: uint32(i), len: 8}
	}
	for i := 144; i < 256; i++ {
		litCodes[i] = prefixCode{sym: uint32(i), len: 9}
	}
	for i := 256; i < 280; i++ {
		litCodes[i] = prefixCode{sym: uint32(i), len: 7}
	}
	for i := 280; i < 288; i++ {
		litCodes[i] = prefixCode{sym: uint32(i), len: 8}
	}
	litTree.Init(litCodes[:], true)

	// These come from the RFC section 3.2.6.
	var distCodes [32]prefixCode
	for i := 0; i < 32; i++ {
		distCodes[i] = prefixCode{sym: uint32(i), len: 5}
	}
	distTree.Init(distCodes[:], true)
}
