// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo

package brotli

import (
	"bytes"
	"hash/crc32"
	"io"
	"path/filepath"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

func TestRoundTrip(t *testing.T) {
	files := []string{
		"binary.bin", "digits.txt", "huffman.txt", "random.bin", "repeats.bin", "twain.txt", "zeros.bin",
	}
	for _, f := range files {
		input := testutil.MustLoadFile(filepath.Join("../../../testdata/", f))

		buf := new(bytes.Buffer)
		zw := NewWriter(buf, 6)
		if _, err := io.Copy(zw, bytes.NewReader(input)); err != nil {
			t.Errorf("test %s, unexpected Copy error: %v", f, err)
			continue
		}
		if err := zw.Close(); err != nil {
			t.Errorf("test %s, unexpected Close error: %v", f, err)
			continue
		}

		hash := crc32.NewIEEE()
		zr := NewReader(buf)
		if _, err := io.Copy(hash, zr); err != nil {
			t.Errorf("test %s, unexpected Copy error: %v", f, err)
			continue
		}
		if err := zr.Close(); err != nil {
			t.Errorf("test %s, unexpected Close error: %v", f, err)
			continue
		}

		if got, want := hash.Sum32(), crc32.ChecksumIEEE(input); got != want {
			t.Errorf("test %s, mismatching checksum: got 0x%08x, want 0x%08x", f, got, want)
		}
	}
}
