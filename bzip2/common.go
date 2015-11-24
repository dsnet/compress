// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package bzip2 implements the BZip2 compressed data format.
package bzip2

import "runtime"
import "hash/crc32"
import "github.com/dsnet/golib/hashutil"

const (
	hdrMagic = "BZ"
	blkMagic = 0x314159265359
	endMagic = 0x177245385090

	magicBits = 48
)

// Error is the wrapper type for errors specific to this library.
type Error string

func (e Error) Error() string { return "bzip2: " + string(e) }

var (
	ErrCorrupt    error = Error("stream is corrupted")
	ErrDeprecated error = Error("deprecated stream format")
)

func errRecover(err *error) {
	switch ex := recover().(type) {
	case nil:
		// Do nothing.
	case runtime.Error:
		panic(ex)
	case error:
		*err = ex
	default:
		panic(ex)
	}
}

var ReverseLUT [256]byte

func init() {
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

// updateCRC returns the result of adding the bytes in buf to the crc.
func updateCRC(crc uint32, buf []byte) uint32 {
	// The CRC-32 computation in bzip2 treats bytes as having bits in big-endian
	// order. That is, the MSB is read before the LSB. Thus, we can use the
	// standard library version of CRC-32 IEEE with some minor adjustments.
	crc = ReverseUint32(crc)
	var arr [4096]byte
	for len(buf) > 0 {
		cnt := copy(arr[:], buf)
		buf = buf[cnt:]
		for i, b := range arr[:cnt] {
			arr[i] = ReverseLUT[b]
		}
		crc = crc32.Update(crc, crc32.IEEETable, arr[:cnt])
	}
	return ReverseUint32(crc)
}

// combineCRC combines two CRC-32 checksums together.
func combineCRC(crc1, crc2 uint32, len2 int64) uint32 {
	crc1 = ReverseUint32(crc1)
	crc2 = ReverseUint32(crc2)
	crc := hashutil.CombineCRC32(crc32.IEEE, crc1, crc2, len2)
	return ReverseUint32(crc)
}
