// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package benchmark compares the performance of various compression
// implementations with respect to encode speed, decode speed, and ratio.
package benchmark

import "io"
import "io/ioutil"

const (
	FormatFlate = iota
	FormatBZ2
	FormatXZ
	FormatBrotli
)

const (
	TestEncodeRate = iota
	TestDecodeRate
	TestCompressRatio
)

type Encoder func(io.Writer, int) io.WriteCloser
type Decoder func(io.Reader) io.ReadCloser

var (
	Encoders map[int]map[string]Encoder
	Decoders map[int]map[string]Decoder
)

func registerEncoder(fmt int, name string, enc Encoder) {
	if Encoders == nil {
		Encoders = make(map[int]map[string]Encoder)
	}
	if Encoders[fmt] == nil {
		Encoders[fmt] = make(map[string]Encoder)
	}
	Encoders[fmt][name] = enc
}

func registerDecoder(fmt int, name string, dec Decoder) {
	if Decoders == nil {
		Decoders = make(map[int]map[string]Decoder)
	}
	if Decoders[fmt] == nil {
		Decoders[fmt] = make(map[string]Decoder)
	}
	Decoders[fmt][name] = dec
}

// LoadFile loads the first n bytes of the input file. If the file is smaller
// than n, then it will replicate the input until it matches n. Each copy will
// be XORed by some mask to avoid favoring algorithms with large LZ77 windows.
func LoadFile(file string, n int) ([]byte, error) {
	input, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if len(input) >= n {
		return input[:n], nil
	}
	if len(input) == 0 {
		return nil, io.ErrNoProgress
	}

	var rb byte // Chunk mask
	output := make([]byte, n)
	buf := output
	for {
		for _, c := range input {
			if len(buf) == 0 {
				return output, nil
			}
			buf[0] = c ^ rb
			buf = buf[1:]
		}
		rb++
	}
}
