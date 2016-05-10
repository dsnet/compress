// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package testutil is a collection of testing helper methods.
package testutil

import (
	"encoding/hex"
	"io"
	"io/ioutil"
)

// LoadFile loads the first n bytes of the input file. If n is less than zero,
// then it will return the input file as is. If the file is smaller than n,
// then it will replicate the input until it matches n. Each copy will be XORed
// by some mask to avoid favoring algorithms with large LZ77 windows.
func LoadFile(file string, n int) ([]byte, error) {
	input, err := ioutil.ReadFile(file)
	switch {
	case err != nil:
		return nil, err
	case n < 0:
		return input, nil
	case len(input) >= n:
		return input[:n], nil
	case len(input) == 0:
		return nil, io.ErrNoProgress // Can't replicate an empty string
	}

	var mask byte
	output := make([]byte, n)
	for i := range output {
		idx := i % len(input)
		output[i] = input[idx] ^ mask
		if idx == len(input)-1 {
			mask++
		}
	}
	return output, nil
}

// MustLoadFile must load a files or else panics.
func MustLoadFile(file string, n int) []byte {
	b, err := LoadFile(file, n)
	if err != nil {
		panic(err)
	}
	return b
}

// MustDecodeHex must decode a hexadecimal string or else panics.
func MustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// MustDecodeBitGen must decode a BitGen formatted string or else panics.
func MustDecodeBitGen(s string) []byte {
	b, err := DecodeBitGen(s)
	if err != nil {
		panic(err)
	}
	return b
}

// BuggyReader returns Err after N bytes have been read from R.
type BuggyReader struct {
	R   io.Reader
	N   int64 // Number of valid bytes to read
	Err error // Return this error after N bytes
}

func (br *BuggyReader) Read(buf []byte) (int, error) {
	if int64(len(buf)) > br.N {
		buf = buf[:br.N]
	}
	n, err := br.R.Read(buf)
	br.N -= int64(n)
	if err == nil && br.N <= 0 {
		return n, br.Err
	}
	return n, err
}

// BuggyWriter returns Err after N bytes have been written to W.
type BuggyWriter struct {
	W   io.Writer
	N   int64 // Number of valid bytes to write
	Err error // Return this error after N bytes
}

func (bw *BuggyWriter) Write(buf []byte) (int, error) {
	if int64(len(buf)) > bw.N {
		buf = buf[:bw.N]
	}
	n, err := bw.W.Write(buf)
	bw.N -= int64(n)
	if err == nil && bw.N <= 0 {
		return n, bw.Err
	}
	return n, err
}
