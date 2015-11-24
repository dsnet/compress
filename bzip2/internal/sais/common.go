// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package sais implements a linear time suffix array algorithm.
package sais

// ComputeSA computes the suffix array of T and places the result in SA.
// Both T and SA must be the same length.
func ComputeSA(T []byte, SA []int) {
	if len(SA) != len(T) {
		panic("mismatching sizes")
	}
	computeSA_byte(T, SA, 0, len(T), 256)
}
