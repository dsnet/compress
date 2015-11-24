// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

// The Burrows-Wheeler Transform implementation used here is based on the
// Suffix Array by Induced Sorting (SA-IS) methodology by Nong, Zhang, and Chan.
// This specific implementation uses the C version written by Yuta Mori.
//
// The SA-IS algorithm runs in O(n) and outputs a Suffix Array. There is a
// mathematical relationship between Suffix Arrays and the Burrow-Wheeler
// Transform, such that a SA can be converted to a BWT in O(n) time.
//
// References:
//	https://sites.google.com/site/yuta256/sais
//	https://github.com/cscott/compressjs/blob/master/lib/BWT.js
//	https://www.quora.com/How-can-I-optimize-burrows-wheeler-transform-and-inverse-transform-to-work-in-O-n-time-O-n-space
//	https://ge-nong.googlecode.com/files/Two%20Efficient%20Algorithms%20for%20Linear%20Time%20Suffix%20Array%20Construction.pdf

import "github.com/dsnet/compress/bzip2/internal/sais"

func encodeBWT(buf []byte) (ptr int) {
	if len(buf) == 0 {
		return -1
	}

	// TODO(dsnet): Find a way to avoid the duplicate input string trick.
	t := make([]byte, 2*len(buf))
	sa := make([]int, 2*len(buf))
	copy(t, buf)
	copy(t[len(buf):], buf)

	sais.ComputeSA(t, sa)

	for i, j := 0, 0; i < 2*len(buf); i++ {
		if idx := int(sa[i]); idx < len(buf) {
			if idx == 0 {
				ptr = j
				idx = len(buf)
			}
			buf[j] = t[idx-1]
			j++
		}
	}
	return ptr
}

func decodeBWT(buf []byte, ptr int) {
	if len(buf) == 0 {
		return
	}

	var c [256]int
	for _, v := range buf {
		c[v]++
	}

	var sum int
	for i, v := range c {
		sum += v
		c[i] = sum - v
	}

	tt := make([]int, len(buf))
	for i := range buf {
		b := buf[i]
		tt[c[b]] |= i
		c[b]++
	}

	buf2 := make([]byte, len(buf))
	tPos := tt[ptr]
	for i := range tt {
		buf2[i] = buf[tPos]
		tPos = tt[tPos]
	}
	copy(buf, buf2)
}
