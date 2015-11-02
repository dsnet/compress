// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build debug

package brotli

import "os"
import "fmt"
import "strings"

const debug = true

func printLUTs() {
	var output = os.Stderr

	printVar := func(name string, obj interface{}) {
		var body string
		if bs, ok := obj.([]uint8); ok && len(bs) >= 256 {
			// Special case handling for large []uint8 to form 16x16 blocks.
			var ss []string
			ss = append(ss, "{")
			var s string
			for i, b := range bs {
				s += fmt.Sprintf("%02x ", b)
				if i%16 == 15 || i+1 == len(bs) {
					ss = append(ss, "\t"+s+"")
					s = ""
				}
				if i%256 == 255 && (i+1 != len(bs)) {
					ss = append(ss, "")
				}
			}
			ss = append(ss, "}")
			body = strings.Join(ss, "\n")
		} else {
			body = fmt.Sprintf("%v", obj)
		}
		fmt.Fprintf(output, "var %s %T = %v\n", name, obj, body)
	}

	// Common LUTs.
	printVar("reverseLUT", reverseLUT[:])
	printVar("mtfLUT", mtfLUT[:])
	fmt.Fprintln(output)

	// Context LUTs.
	printVar("contextP1LUT", contextP1LUT[:])
	printVar("contextP2LUT", contextP2LUT[:])
	fmt.Fprintln(output)

	// Static dictionary LUTs.
	printVar("dictBitSizes", dictBitSizes)
	printVar("dictSizes", dictSizes)
	printVar("dictOffsets", dictOffsets)
	fmt.Fprintln(output)

	// Prefix LUTs.
	printVar("simpleLens1", simpleLens1)
	printVar("simpleLens2", simpleLens2)
	printVar("simpleLens3", simpleLens3)
	printVar("simpleLens4a", simpleLens4a)
	printVar("simpleLens4b", simpleLens4b)
	printVar("complexLens", complexLens)
	fmt.Fprintln(output)

	printVar("insLenRanges", insLenRanges)
	printVar("cpyLenRanges", cpyLenRanges)
	printVar("blkLenRanges", blkLenRanges)
	printVar("maxRLERanges", maxRLERanges)
	fmt.Fprintln(output)

	printVar("codeCLens", codeCLens)
	printVar("decCLens", decCLens)
	printVar("encCLens", encCLens)
	fmt.Fprintln(output)

	printVar("codeMaxRLE", codeMaxRLE)
	printVar("decMaxRLE", decMaxRLE)
	printVar("encMaxRLE", encMaxRLE)
	fmt.Fprintln(output)

	printVar("codeWinBits", codeWinBits)
	printVar("decWinBits", decWinBits)
	printVar("encWinBits", encWinBits)
	fmt.Fprintln(output)

	printVar("codeCounts", codeCounts)
	printVar("decCounts", decCounts)
	printVar("encCounts", encCounts)
	fmt.Fprintln(output)
}

func (rc rangeCodes) String() (s string) {
	var maxBits, maxBase int
	for _, c := range rc {
		if maxBits < int(c.bits) {
			maxBits = int(c.bits)
		}
		if maxBase < int(c.base) {
			maxBase = int(c.base)
		}
	}

	var ss []string
	ss = append(ss, "{")
	maxSymDig := len(fmt.Sprintf("%d", len(rc)-1))
	maxBitsDig := len(fmt.Sprintf("%d", maxBits))
	maxBaseDig := len(fmt.Sprintf("%d", maxBase))
	for i, c := range rc {
		base := fmt.Sprintf(fmt.Sprintf("%%%dd", maxBaseDig), c.base)
		if c.bits > 0 {
			base += fmt.Sprintf("-%d", c.base+1<<c.bits-1)
		}
		ss = append(ss, fmt.Sprintf(
			fmt.Sprintf("\t%%%dd:  {bits: %%%dd, base: %%s},",
				maxSymDig, maxBitsDig),
			i, c.bits, base,
		))
	}
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}

func (pc prefixCodes) String() (s string) {
	var maxSym, maxLen int
	for _, c := range pc {
		if maxSym < int(c.sym) {
			maxSym = int(c.sym)
		}
		if maxLen < int(c.len) {
			maxLen = int(c.len)
		}
	}

	var ss []string
	ss = append(ss, "{")
	maxSymDig := len(fmt.Sprintf("%d", maxSym))
	for _, c := range pc {
		ss = append(ss, fmt.Sprintf(
			fmt.Sprintf("\t%%%dd:%s%%0%db,",
				maxSymDig, strings.Repeat(" ", 2+maxLen-int(c.len)), c.len),
			c.sym, c.val,
		))
	}
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}

func (pd prefixDecoder) String() string {
	var ss []string
	ss = append(ss, "{")
	if len(pd.chunks) > 0 {
		ss = append(ss, "\tchunks: {")
		for i, c := range pd.chunks {
			l := "sym"
			if uint(c&prefixCountMask) > uint(pd.chunkBits) {
				l = "idx"
			}
			ss = append(ss, fmt.Sprintf(
				fmt.Sprintf("\t\t%%0%db:  {%%s: %%3d, len: %%2d},", pd.chunkBits),
				i, l, c>>prefixCountBits, c&prefixCountMask,
			))
		}
		ss = append(ss, "\t},")

		for j, links := range pd.links {
			ss = append(ss, fmt.Sprintf("\tlinks[%d]: {", j))
			linkBits := len(fmt.Sprintf("%b", pd.linkMask))
			for i, c := range links {
				ss = append(ss, fmt.Sprintf(
					fmt.Sprintf("\t\t%%0%db:  {sym: %%3d, len: %%2d},", linkBits),
					i, c>>prefixCountBits, c&prefixCountMask,
				))
			}
			ss = append(ss, "\t},")
		}
		ss = append(ss, fmt.Sprintf("\tchunkMask: %b,", pd.chunkMask))
		ss = append(ss, fmt.Sprintf("\tlinkMask: %b,", pd.linkMask))
		ss = append(ss, fmt.Sprintf("\tnumSyms: %d,", pd.numSyms))
		ss = append(ss, fmt.Sprintf("\tchunkBits: %d,", pd.chunkBits))
		ss = append(ss, fmt.Sprintf("\tminBits: %d,", pd.minBits))
	}
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}
