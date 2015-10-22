// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

// TODO(dsnet): Some of this logic is identical to compress/flate.
// Centralize common logic to compress/internal/prefix.

const (
	// These constants are from RFC section 3.3.
	numLitSyms = 256
	numInsSyms = 704
	numCntSyms = 26

	// The largest alphabet size of any prefix table from RFC section 3.3.
	// Thus, it is okay to use uint16 to store symbols.
	maxAlphabetLen = numInsSyms

	// This is the maximum bit-width of a prefix code from RFC section 3.5.
	// Thus, it is okay to use uint16 to store codes.
	maxPrefixLen = 15
)

var (
	// RFC section 3.4.
	// Prefix codes for simple codes.
	simpleLens1  = [1]uint8{0}
	simpleLens2  = [2]uint8{1, 1}
	simpleLens3  = [3]uint8{1, 2, 2}
	simpleLens4a = [4]uint8{2, 2, 2, 2}
	simpleLens4b = [4]uint8{1, 2, 3, 3}

	// RFC section 3.5.
	// Array of prefix lengths in the order they appear in the stream.
	codeLens = [18]uint16{1, 2, 3, 4, 0, 5, 17, 6, 16, 7, 8, 9, 10, 11, 12, 13, 14, 15}
)

var (
	// RFC section 3.5.
	// Prefix codecs for code lengths in complex prefix definition.
	decCodeLens prefixDecoder
	encCodeLens prefixEncoder

	// RFC section 7.3.
	// Prefix codecs for RLEMAX in context map definition.
	decMaxRLE prefixDecoder
	encMaxRLE prefixEncoder

	// RFC section 9.1.
	// Prefix codecs for WBITS in stream header definition.
	decWinBits prefixDecoder
	encWinBits prefixEncoder

	// RFC section 9.2.
	// Prefix codecs used for size fields in meta-block header definition.
	decCounts prefixDecoder
	encCounts prefixEncoder
)

type prefixCode struct {
	sym uint16 // The symbol being mapped
	val uint16 // Value of the prefix code (must be in [0..1<<len])
	len uint8  // Bit length of the prefix code
}

func initPrefixLUTs() {
	if maxAlphabetLen > 1<<(16-prefixCountBits) {
		panic("maximum alphabet size is too large to represent")
	}
	if maxPrefixLen > 1<<prefixCountBits {
		panic("maximum prefix bit-length is too large to represent")
	}

	// Prefix code for reading code lengths in RFC section 3.5.
	var clenCodes []prefixCode
	for sym, clen := range []uint8{2, 4, 3, 2, 2, 4} {
		clenCodes = append(clenCodes, prefixCode{sym: uint16(sym), len: clen})
	}
	decCodeLens.Init(clenCodes, true)
	encCodeLens.Init(clenCodes)

	// Prefix code for reading RLEMAX in RFC section 7.3.
	var rleCodes = []prefixCode{{sym: 0, val: 0, len: 1}}
	for i := uint16(0); i < 16; i++ {
		rleCodes = append(rleCodes, prefixCode{
			sym: i + 1,
			val: i<<1 | 1,
			len: 5,
		})
	}
	decMaxRLE.Init(rleCodes, false)
	encMaxRLE.Init(rleCodes)

	// Prefix code for reading WBITS in RFC section 9.1.
	var winCodes []prefixCode
	for i := uint16(9); i <= 24; i++ {
		var code prefixCode
		switch {
		case i < 16:
			code = prefixCode{sym: i, val: i - 8, len: 7}
			if i == 9 {
				code.sym = 0 // Invalid code "1000100"
			}
		case i == 16:
			code = prefixCode{sym: i, val: i - 16, len: 1}
		case i > 16:
			code = prefixCode{sym: i, val: i - 17, len: 4}
			if i == 17 {
				code.len = 7 // Symbol 17 is oddly longer
			}
		}
		if code.len > 1 {
			code.val = code.val<<(code.len-3) | 1
		}
		winCodes = append(winCodes, code)
	}
	decWinBits.Init(winCodes, false)
	encWinBits.Init(winCodes)

	// Prefix code for reading counts in RFC section 9.2.
	// This is used for: NBLTYPESL, NBLTYPESI, NBLTYPESD, NTREESL, and NTREESD.
	var cntCodes = []prefixCode{{sym: 1, val: 0, len: 1}}
	for i := uint16(0); i < 8; i++ {
		for j := uint16(0); j < 1<<uint(i); j++ {
			cntCodes = append(cntCodes, prefixCode{
				sym: cntCodes[len(cntCodes)-1].sym + 1,
				val: j<<4 | i<<1 | 1,
				len: uint8(i + 4),
			})
		}
	}
	decCounts.Init(cntCodes, false)
	encCounts.Init(cntCodes)
}
