// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package bzip2 implements the BZip2 compressed data format.
package bzip2

import "runtime"
import "hash/crc32"
import "github.com/dsnet/compress/bzip2/internal"
import "github.com/dsnet/golib/hashutil"

// There does not exist a formal specification of the BZip2 format. As such,
// much of this work is derived by either reverse engineering the original C
// source code or using secondary sources.
//
// Compression stack:
//	Run-length encoding 1     (RLE1)
//	Burrows-Wheeler transform (BWT)
//	Move-to-front transform   (MTF)
//	Run-length encoding 2     (RLE2)
//	Prefix encoding           (PE)
//
// References:
//	http://bzip.org/
//	https://en.wikipedia.org/wiki/Bzip2
//	https://code.google.com/p/jbzip2/

const (
	hdrMagic = "BZ"
	blkMagic = 0x314159265359 // BCD of PI
	endMagic = 0x177245385090 // BCD of sqrt(PI)

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

// updateCRC returns the result of adding the bytes in buf to the crc.
func updateCRC(crc uint32, buf []byte) uint32 {
	// The CRC-32 computation in bzip2 treats bytes as having bits in big-endian
	// order. That is, the MSB is read before the LSB. Thus, we can use the
	// standard library version of CRC-32 IEEE with some minor adjustments.
	crc = internal.ReverseUint32(crc)
	var arr [4096]byte
	for len(buf) > 0 {
		cnt := copy(arr[:], buf)
		buf = buf[cnt:]
		for i, b := range arr[:cnt] {
			arr[i] = internal.ReverseLUT[b]
		}
		crc = crc32.Update(crc, crc32.IEEETable, arr[:cnt])
	}
	return internal.ReverseUint32(crc)
}

// combineCRC combines two CRC-32 checksums together.
func combineCRC(crc1, crc2 uint32, len2 int64) uint32 {
	crc1 = internal.ReverseUint32(crc1)
	crc2 = internal.ReverseUint32(crc2)
	crc := hashutil.CombineCRC32(crc32.IEEE, crc1, crc2, len2)
	return internal.ReverseUint32(crc)
}
