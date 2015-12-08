// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build debug

package prefix

import (
	"fmt"
	"strings"
)

func lenBase2(n interface{}) int { return len(fmt.Sprintf("%b", n)) }
func padBase2(v, n interface{}, m int) string {
	var s string
	if fmt.Sprint(n) != "0" {
		s = fmt.Sprintf(fmt.Sprintf("%%0%db", n), v)
	}
	if pad := m - len(s); pad > 0 {
		s = strings.Repeat(" ", pad) + s
	}
	return s
}

func lenBase10(n int) int { return len(fmt.Sprintf("%d", n)) }
func padBase10(n interface{}, m int) string {
	s := fmt.Sprintf("%d", n)
	if pad := m - len(s); pad > 0 {
		s = strings.Repeat(" ", pad) + s
	}
	return s
}

func (rc RangeCodes) String() string {
	var maxLen, maxBase int
	for _, c := range rc {
		if maxLen < int(c.Len) {
			maxLen = int(c.Len)
		}
		if maxBase < int(c.Base) {
			maxBase = int(c.Base)
		}
	}
	maxSymStr := lenBase10(len(rc) - 1)
	maxLenStr := lenBase10(maxLen)
	maxBaseStr := lenBase10(maxBase)

	var ss []string
	ss = append(ss, "{")
	for i, c := range rc {
		base := fmt.Sprintf(fmt.Sprintf("%%%dd", maxBaseStr), c.Base)
		if c.Len > 0 {
			base += fmt.Sprintf("-%d", c.End()-1)
		}
		ss = append(ss, fmt.Sprintf(
			fmt.Sprintf("\t%%%dd:  {bits: %%%dd, base: %%s},",
				maxSymStr, maxLenStr),
			i, c.Len, base,
		))
	}
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}

func (pc PrefixCodes) String() string {
	var maxSym, maxLen, maxCnt int
	for _, c := range pc {
		if maxSym < int(c.Sym) {
			maxSym = int(c.Sym)
		}
		if maxLen < int(c.Len) {
			maxLen = int(c.Len)
		}
		if maxCnt < int(c.Cnt) {
			maxCnt = int(c.Cnt)
		}
	}
	maxSymStr := lenBase10(maxSym)
	maxCntStr := lenBase10(maxCnt)

	var ss []string
	ss = append(ss, "{")
	for _, c := range pc {
		var cntStr string
		if maxCnt > 0 {
			cnt := int(32*float32(c.Cnt)/float32(maxCnt) + 0.5)
			cntStr = fmt.Sprintf("%s |%s",
				padBase10(c.Cnt, maxCntStr),
				strings.Repeat("#", cnt),
			)
		}
		ss = append(ss, fmt.Sprintf("\t%s:  %s,  %s",
			padBase10(c.Sym, maxSymStr),
			padBase2(c.Val, c.Len, maxLen),
			cntStr,
		))
	}
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}

func (pd Decoder) String() string {
	var ss []string
	ss = append(ss, "{")
	if len(pd.chunks) > 0 {
		ss = append(ss, "\tchunks: {")
		for i, c := range pd.chunks {
			l := "sym"
			if uint(c&countMask) > uint(pd.chunkBits) {
				l = "idx"
			}
			ss = append(ss, fmt.Sprintf("\t\t%s:  {%s: %s, len: %s}",
				padBase2(i, pd.chunkBits, int(pd.chunkBits)),
				l, padBase10(c>>countBits, 3),
				padBase10(c&countMask, 2),
			))
		}
		ss = append(ss, "\t},")

		for j, links := range pd.links {
			ss = append(ss, fmt.Sprintf("\tlinks[%d]: {", j))
			linkBits := lenBase2(pd.linkMask)
			for i, c := range links {
				ss = append(ss, fmt.Sprintf("\t\t%s:  {sym: %s, len: %s},",
					padBase2(i, linkBits, int(linkBits)),
					padBase10(c>>countBits, 3),
					padBase10(c&countMask, 2),
				))
			}
			ss = append(ss, "\t},")
		}
	}
	ss = append(ss, fmt.Sprintf("\tchunkMask: %b,", pd.chunkMask))
	ss = append(ss, fmt.Sprintf("\tlinkMask: %b,", pd.linkMask))
	ss = append(ss, fmt.Sprintf("\tchunkBits: %d,", pd.chunkBits))
	ss = append(ss, fmt.Sprintf("\tminBits: %d,", pd.minBits))
	ss = append(ss, fmt.Sprintf("\tnumSyms: %d,", pd.numSyms))
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}

func (pe Encoder) String() string {
	var maxLen int
	for _, c := range pe.chunks {
		if maxLen < int(c&countMask) {
			maxLen = int(c & countMask)
		}
	}

	var ss []string
	ss = append(ss, "{")
	if len(pe.chunks) > 0 {
		ss = append(ss, "\tchunks: {")
		for i, c := range pe.chunks {
			ss = append(ss, fmt.Sprintf("\t\t%s:  %s,",
				padBase10(i, 3),
				padBase2(c>>countBits, c&countMask, maxLen),
			))
		}
		ss = append(ss, "\t},")
	}
	ss = append(ss, fmt.Sprintf("\tchunkMask: %b,", pe.chunkMask))
	ss = append(ss, fmt.Sprintf("\tnumSyms: %d,", pe.numSyms))
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}
