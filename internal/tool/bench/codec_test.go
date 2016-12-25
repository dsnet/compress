// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bench

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

// TestCodecs tests that the output of each registered encoder is a valid input
// for each registered decoder. This test runs in O(n^2) where n is the number
// of registered codecs. This assumes that the number of test files and
// compression formats stays relatively constant.
func TestCodecs(t *testing.T) {
	files := []string{
		"binary.bin", "digits.txt", "huffman.txt", "random.bin", "repeats.bin", "twain.txt", "zeros.bin",
	}
	for _, fl := range files {
		dd := testutil.MustLoadFile(filepath.Join("../../../testdata", fl))
		t.Run(fmt.Sprintf("File:%v", fl), func(t *testing.T) { testFormats(t, dd) })
	}
}

func testFormats(t *testing.T, dd []byte) {
	t.Parallel()
	formats := []Format{
		FormatFlate, FormatBrotli, FormatBZ2, FormatLZMA2, FormatZstd,
	}
	for _, ft := range formats {
		if len(Encoders[ft]) == 0 || len(Decoders[ft]) == 0 {
			t.Skip("no codecs available")
		}
		t.Run(fmt.Sprintf("Format:%v", ft), func(t *testing.T) { testEncoders(t, ft, dd) })
	}
}

func testEncoders(t *testing.T, ft Format, dd []byte) {
	t.Parallel()
	const level = 6 // Default compression on all encoders
	for encName := range Encoders[ft] {
		encName := encName
		t.Run(fmt.Sprintf("Encoder:%v", encName), func(t *testing.T) {
			be := new(bytes.Buffer)
			zw := Encoders[ft][encName](be, level)
			if _, err := io.Copy(zw, bytes.NewReader(dd)); err != nil {
				t.Fatalf("unexpected Write error: %v", err)
			}
			if err := zw.Close(); err != nil {
				t.Fatalf("unexpected Close error: %v", err)
			}
			de := be.Bytes()
			testDecoders(t, ft, dd, de)
		})
	}
}

func testDecoders(t *testing.T, ft Format, dd, de []byte) {
	t.Parallel()
	for decName := range Decoders[ft] {
		decName := decName
		t.Run(fmt.Sprintf("Decoder:%v", decName), func(t *testing.T) {
			bd := new(bytes.Buffer)
			zr := Decoders[ft][decName](bytes.NewReader(de))
			if _, err := io.Copy(bd, zr); err != nil {
				t.Fatalf("unexpected Read error: %v", err)
			}
			if err := zr.Close(); err != nil {
				t.Fatalf("unexpected Close error: %v", err)
			}
			if !bytes.Equal(bd.Bytes(), dd) {
				t.Error("data mismatch")
			}
		})
	}
}
