// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package sais implements a linear time suffix array algorithm.
package sais

// This package ports the C sais implementation by Yuta Mori. The ports are
// located in sais_byte.go and sais_int.go. Since Go does not support generics,
// the implementations are copy and pastes of each other with minor changes
// to the types used.
//
// The sais_int.go file can be generated from sais_byte.go with the the
// following search-and-replace transformation:
//	s/byte/int/g
//
// References:
//	https://sites.google.com/site/yuta256/sais
//	https://ge-nong.googlecode.com/files/Linear%20Time%20Suffix%20Array%20Construction%20Using%20D-Critical%20Substrings.pdf
//	https://ge-nong.googlecode.com/files/Two%20Efficient%20Algorithms%20for%20Linear%20Time%20Suffix%20Array%20Construction.pdf

// ComputeSA computes the suffix array of T and places the result in SA.
// Both T and SA must be the same length.
func ComputeSA(T []byte, SA []int) {
	if len(SA) != len(T) {
		panic("mismatching sizes")
	}
	computeSA_byte(T, SA, 0, len(T), 256)
}
