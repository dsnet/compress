// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import "io"
import "github.com/dsnet/golib/bits"

// Divide n by m and round up to the nearest multiple of m.
func divCeil(n, m int) int {
	return (n + m - 1) / m
}

// Number of bits needed to pad n-bits to a byte alignment.
func numPads(n int) int {
	return divCeil(n, 8)*8 - n
}

// Compute the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Compute the maximum of two integers.
func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// Convert a boolean to a integer sign.
func sign(b bool) int {
	if b {
		return +1
	}
	return -1
}

// Read multiple bits.
// This function panics if an error occurs.
func readBits(br bits.BitsReader, num int) uint {
	val, _, err := br.ReadBits(num)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		panic(err)
	}
	return val
}
