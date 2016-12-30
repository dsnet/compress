// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

package bzip2

import (
	"bytes"
	"io/ioutil"

	"github.com/dsnet/compress"
	gbzip2 "github.com/dsnet/compress/bzip2"
	cbzip2 "github.com/dsnet/compress/internal/cgo/bzip2"
)

func Fuzz(data []byte) int {
	data, ok := testDecoders(data)
	for i := 1; i <= 9; i++ {
		testGoEncoder(data, i)
		testCEncoder(data, i)
	}
	if ok {
		return 1 // Favor valid inputs
	}
	return 0
}

// testDecoders tests that the input can be handled by both Go and C decoders.
// This test does not panic if both decoders run into an error, since it
// means that they both agree that the input is bad.
func testDecoders(data []byte) ([]byte, bool) {
	gr, err := gbzip2.NewReader(bytes.NewReader(data), nil)
	if err != nil {
		panic(err)
	}
	defer gr.Close()
	cr := cbzip2.NewReader(bytes.NewReader(data))
	defer cr.Close()

	gb, gerr := ioutil.ReadAll(gr)
	cb, cerr := ioutil.ReadAll(cr)

	switch {
	case gerr == nil && cerr == nil:
		if !bytes.Equal(gb, cb) {
			panic("mismatching bytes")
		}
		if err := gr.Close(); err != nil {
			panic(err)
		}
		if err := cr.Close(); err != nil {
			panic(err)
		}
		return gb, true
	case gerr != nil && cerr == nil:
		if err, ok := gerr.(compress.Error); ok && err.IsDeprecated() {
			return cb, false
		}
		panic(gerr)
	case gerr == nil && cerr != nil:
		panic(cerr)
	default:
		return nil, false
	}
}

// testGoEncoder encodes the input data with the Go encoder and then checks that
// both the Go and C decoders can properly decompress the output.
func testGoEncoder(data []byte, level int) {
	// Compress using the Go encoder.
	bb := new(bytes.Buffer)
	gw, err := gbzip2.NewWriter(bb, &gbzip2.WriterConfig{Level: level})
	if err != nil {
		panic(err)
	}
	defer gw.Close()
	n, err := gw.Write(data)
	if n != len(data) || err != nil {
		panic(err)
	}
	if err := gw.Close(); err != nil {
		panic(err)
	}

	// Decompress using both the Go and C decoders.
	b, ok := testDecoders(bb.Bytes())
	if !ok {
		panic("decoder error")
	}
	if !bytes.Equal(b, data) {
		panic("mismatching bytes")
	}
}

// testCEncoder encodes the input data with the C encoder and then checks that
// both the Go and C decoders can properly decompress the output.
func testCEncoder(data []byte, level int) {
	// Compress using the C encoder.
	bb := new(bytes.Buffer)
	cw := cbzip2.NewWriter(bb, level)
	defer cw.Close()
	n, err := cw.Write(data)
	if n != len(data) || err != nil {
		panic(err)
	}
	if err := cw.Close(); err != nil {
		panic(err)
	}

	// Decompress using both the Go and C decoders.
	b, ok := testDecoders(bb.Bytes())
	if !ok {
		panic("decoder error")
	}
	if !bytes.Equal(b, data) {
		panic("mismatching bytes")
	}
}
