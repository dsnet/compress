// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package internal is a collection of common compression algorithms.
//
// For performance reasons, these packages lack strong error checking and
// require that the caller to ensure that strict invariants are kept.
package internal

// Error is the wrapper type for errors specific to this library.
type Error string

func (e Error) Error() string { return "compress: " + string(e) }

var (
	// IdentityLUT returns the input key itself.
	IdentityLUT [256]byte

	// ReverseLUT returns the input key with its bits reversed.
	ReverseLUT [256]byte
)

func init() {
	for i := range IdentityLUT {
		IdentityLUT[i] = uint8(i)
	}
	for i := range ReverseLUT {
		b := uint8(i)
		b = (b&0xaa)>>1 | (b&0x55)<<1
		b = (b&0xcc)>>2 | (b&0x33)<<2
		b = (b&0xf0)>>4 | (b&0x0f)<<4
		ReverseLUT[i] = b
	}
}

// ReverseUint32 reverses all bits of v.
func ReverseUint32(v uint32) (x uint32) {
	x |= uint32(ReverseLUT[byte(v>>0)]) << 24
	x |= uint32(ReverseLUT[byte(v>>8)]) << 16
	x |= uint32(ReverseLUT[byte(v>>16)]) << 8
	x |= uint32(ReverseLUT[byte(v>>24)]) << 0
	return x
}

// ReverseUint32N reverses the lower n bits of v.
func ReverseUint32N(v uint32, n uint) (x uint32) {
	return uint32(ReverseUint32(uint32(v << (32 - n))))
}
