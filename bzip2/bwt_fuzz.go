// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

// This file exists to export internal implementation details for fuzz testing.

package bzip2

func ForwardBWT(buf []byte) (ptr int) {
	var bwt burrowsWheelerTransform
	return bwt.Encode(buf)
}

func ReverseBWT(buf []byte, ptr int) {
	var bwt burrowsWheelerTransform
	bwt.Decode(buf, ptr)
}
