// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package brotli implements the Brotli compressed data format.
package brotli

import "runtime"

// Error is the wrapper type for errors specific to this library.
type Error string

func (e Error) Error() string { return "brotli: " + string(e) }

var (
	ErrCorrupt  = Error("stream is corrupted")
	ErrInvalid  = Error("cannot encode data")
	ErrInternal = Error("internal error")
)

func errRecover(err *error) {
	switch ex := recover().(type) {
	case nil:
		// Do nothing
	case runtime.Error:
		panic(ex)
	case error:
		*err = ex
	default:
		panic(ex)
	}
}

var reverseLUT []uint8

func initCommonLUTs() {
	reverseLUT = make([]uint8, 256)
	for i := range reverseLUT {
		b := uint8(i)
		b = (b&0xaa)>>1 | (b&0x55)<<1
		b = (b&0xcc)>>2 | (b&0x33)<<2
		b = (b&0xf0)>>4 | (b&0x0f)<<4
		reverseLUT[i] = b
	}
}

// reverseUint16 reverses all bits of v.
func reverseUint16(v uint16) uint16 {
	return uint16(reverseLUT[v>>8]) | uint16(reverseLUT[v&0xff])<<8
}

// reverseBits reverses the lower n bits of v.
func reverseBits(v uint16, n uint) uint16 {
	return reverseUint16(v << uint8(16-n))
}

func initLUTs() {
	initCommonLUTs()
	initPrefixLUTs()
	initContextLUTs()
	initDictLUTs()
}

func init() { initLUTs() }

