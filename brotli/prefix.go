// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

// TODO(dsnet): Some of this logic is identical to compress/flate.
// Centralize common logic to compress/internal/prefix.

const (
	// RFC section 3.5.
	// This is the maximum bit-width of a prefix code.
	// Thus, it is okay to use uint16 to store codes.
	maxPrefixBits = 15

	// RFC section 3.3.
	// The size of the alphabet for various prefix codes.
	numLitSyms        = 256                  // Literal symbols
	maxNumDistSyms    = 16 + 120 + (48 << 3) // Distance symbols
	numInsSyms        = 704                  // Insert-and-copy length symbols
	numBlkCntSyms     = 26                   // Block count symbols
	maxNumBlkTypeSyms = 256 + 2              // Block type symbols
	maxNumCtxMapSyms  = 256 + 16             // Context map symbols

	// This should be the max of each of the constants above.
	maxNumAlphabetSyms = numInsSyms
)

var (
	// RFC section 3.4.
	// Prefix code lengths for simple codes.
	simpleLens1  = [1]uint{0}
	simpleLens2  = [2]uint{1, 1}
	simpleLens3  = [3]uint{1, 2, 2}
	simpleLens4a = [4]uint{2, 2, 2, 2}
	simpleLens4b = [4]uint{1, 2, 3, 3}

	// RFC section 3.5.
	// Prefix code lengths for complex codes as they appear in the stream.
	complexLens = [18]uint{
		1, 2, 3, 4, 0, 5, 17, 6, 16, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	}
)

type rangeCode struct {
	base uint32 // Starting base offset of the range
	bits uint8  // Bit-width of a subsequent integer to add to base offset
}
type rangeCodes []rangeCode

var (
	// RFC section 5.
	// LUT to convert an insert symbol to an actual insert length.
	insLenRanges rangeCodes

	// RFC section 5.
	// LUT to convert an copy symbol to an actual copy length.
	cpyLenRanges rangeCodes

	// RFC section 6.
	// LUT to convert an block-type length symbol to an actual length.
	blkLenRanges rangeCodes

	// RFC section 7.3.
	// LUT to convert RLE symbol to an actual repeat length.
	maxRLERanges rangeCodes
)

type prefixCode struct {
	sym uint16 // The symbol being mapped
	val uint16 // Value of the prefix code (must be in [0..1<<len])
	len uint8  // Bit length of the prefix code
}
type prefixCodes []prefixCode

var (
	// RFC section 3.5.
	// Prefix codecs for code lengths in complex prefix definition.
	codeCLens prefixCodes
	decCLens  prefixDecoder
	encCLens  prefixEncoder

	// RFC section 7.3.
	// Prefix codecs for RLEMAX in context map definition.
	codeMaxRLE prefixCodes
	decMaxRLE  prefixDecoder
	encMaxRLE  prefixEncoder

	// RFC section 9.1.
	// Prefix codecs for WBITS in stream header definition.
	codeWinBits prefixCodes
	decWinBits  prefixDecoder
	encWinBits  prefixEncoder

	// RFC section 9.2.
	// Prefix codecs used for size fields in meta-block header definition.
	codeCounts prefixCodes
	decCounts  prefixDecoder
	encCounts  prefixEncoder
)

func initPrefixLUTs() {
	// Sanity check some constants.
	for _, numMax := range []uint{
		numLitSyms, maxNumDistSyms, numInsSyms, numBlkCntSyms, maxNumBlkTypeSyms, maxNumCtxMapSyms,
	} {
		if numMax > maxNumAlphabetSyms {
			panic("maximum alphabet size is not updated")
		}
	}
	if maxNumAlphabetSyms >= 1<<prefixSymbolBits {
		panic("maximum alphabet size is too large to represent")
	}
	if maxPrefixBits >= 1<<prefixCountBits {
		panic("maximum prefix bit-length is too large to represent")
	}

	initPrefixRangeLUTs()
	initPrefixCodeLUTs()
}

func initPrefixRangeLUTs() {
	var makeRanges = func(base uint, bits []uint) (rc []rangeCode) {
		for _, nb := range bits {
			rc = append(rc, rangeCode{base: uint32(base), bits: uint8(nb)})
			base += 1 << nb
		}
		return rc
	}

	insLenRanges = makeRanges(0, []uint{
		0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 12, 14, 24,
	}) // RFC section 5
	cpyLenRanges = makeRanges(2, []uint{
		0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 24,
	}) // RFC section 5
	blkLenRanges = makeRanges(1, []uint{
		2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 7, 8, 9, 10, 11, 12, 13, 24,
	}) // RFC section 6
	maxRLERanges = makeRanges(2, []uint{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
	}) // RFC section 7.3
}

func initPrefixCodeLUTs() {
	// Prefix code for reading code lengths in RFC section 3.5.
	codeCLens = nil
	for sym, clen := range []uint{2, 4, 3, 2, 2, 4} {
		var code = prefixCode{sym: uint16(sym), len: uint8(clen)}
		codeCLens = append(codeCLens, code)
	}
	decCLens.Init(codeCLens, true)
	encCLens.Init(codeCLens)

	// Prefix code for reading RLEMAX in RFC section 7.3.
	codeMaxRLE = []prefixCode{{sym: 0, val: 0, len: 1}}
	for i := uint16(0); i < 16; i++ {
		var code = prefixCode{sym: i + 1, val: i<<1 | 1, len: 5}
		codeMaxRLE = append(codeMaxRLE, code)
	}
	decMaxRLE.Init(codeMaxRLE, false)
	encMaxRLE.Init(codeMaxRLE)

	// Prefix code for reading WBITS in RFC section 9.1.
	codeWinBits = nil
	for i := uint16(9); i <= 24; i++ {
		var code prefixCode
		switch {
		case i == 16:
			code = prefixCode{sym: i, val: (i-16)<<0 | 0, len: 1} // Symbols: 16
		case i > 17:
			code = prefixCode{sym: i, val: (i-17)<<1 | 1, len: 4} // Symbols: 18..24
		case i < 17:
			code = prefixCode{sym: i, val: (i-8)<<4 | 1, len: 7} // Symbols: 9..15
		default:
			code = prefixCode{sym: i, val: (i-17)<<4 | 1, len: 7} // Symbols: 17
		}
		codeWinBits = append(codeWinBits, code)
	}
	codeWinBits[0].sym = 0 // Invalid code "1000100" to use symbol zero
	decWinBits.Init(codeWinBits, false)
	encWinBits.Init(codeWinBits)

	// Prefix code for reading counts in RFC section 9.2.
	// This is used for: NBLTYPESL, NBLTYPESI, NBLTYPESD, NTREESL, and NTREESD.
	codeCounts = []prefixCode{{sym: 1, val: 0, len: 1}}
	var code = codeCounts[len(codeCounts)-1]
	for i := uint16(0); i < 8; i++ {
		for j := uint16(0); j < 1<<i; j++ {
			code.sym = code.sym + 1
			code.val = j<<4 | i<<1 | 1
			code.len = uint8(i + 4)
			codeCounts = append(codeCounts, code)
		}
	}
	decCounts.Init(codeCounts, false)
	encCounts.Init(codeCounts)
}
