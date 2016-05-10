// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	// TODO(dsnet): We should not be relying on the standard library for the
	// round-trip test.
	"compress/flate"

	"github.com/dsnet/compress"
	"github.com/dsnet/compress/internal/testutil"
)

const (
	binary  = "../testdata/binary.bin"
	digits  = "../testdata/digits.txt"
	huffman = "../testdata/huffman.txt"
	random  = "../testdata/random.bin"
	repeats = "../testdata/repeats.bin"
	twain   = "../testdata/twain.txt"
	zeros   = "../testdata/zeros.bin"
)

func TestRoundTrip(t *testing.T) {
	var vectors = []struct{ input []byte }{
		{nil},
		{testutil.MustLoadFile(binary, -1)},
		{testutil.MustLoadFile(digits, -1)},
		{testutil.MustLoadFile(huffman, -1)},
		{testutil.MustLoadFile(random, -1)},
		{testutil.MustLoadFile(repeats, -1)},
		{testutil.MustLoadFile(twain, -1)},
		{testutil.MustLoadFile(zeros, -1)},
	}

	for i, v := range vectors {
		var buf bytes.Buffer

		// Compress the input.
		wr, _ := flate.NewWriter(&buf, flate.DefaultCompression)
		cnt, err := io.Copy(wr, bytes.NewReader(v.input))
		if err != nil {
			t.Errorf("test %d, write error: got %v", i, err)
		}
		if cnt != int64(len(v.input)) {
			t.Errorf("test %d, write count mismatch: got %d, want %d", i, cnt, len(v.input))
		}
		if err := wr.Close(); err != nil {
			t.Errorf("test %d, close error: got %v", i, err)
		}

		// Write a canary byte to ensure this does not get read.
		buf.WriteByte(0x7a)

		// Decompress the output.
		rd, err := NewReader(&struct{ compress.ByteReader }{&buf}, nil)
		if err != nil {
			t.Errorf("test %d, NewReader error: got %v", i, err)
		}
		output, err := ioutil.ReadAll(rd)
		if err != nil {
			t.Errorf("test %d, read error: got %v", i, err)
		}
		if !bytes.Equal(output, v.input) {
			t.Errorf("test %d, output data mismatch", i)
		}
		if err := wr.Close(); err != nil {
			t.Errorf("test %d, close error: got %v", i, err)
		}

		// Read back the canary byte.
		if v, _ := buf.ReadByte(); v != 0x7a {
			t.Errorf("Read consumed more data than necessary")
		}
	}
}

// TestSync tests that the Reader can read all data compressed thus far by the
// Writer once Flush is called.
func TestSync(t *testing.T) {
	const prime = 13
	var flushSizes []int
	for i := 1; i < 1000; i++ {
		flushSizes = append(flushSizes, i)
	}
	for i := 1; i <= 1<<16; i *= 2 {
		flushSizes = append(flushSizes, i)
		flushSizes = append(flushSizes, i+prime)
	}
	for i := 1; i <= 10000; i *= 10 {
		flushSizes = append(flushSizes, i)
		flushSizes = append(flushSizes, i+prime)
	}

	// Load test data of sufficient size.
	var maxSize, totalSize int
	for _, n := range flushSizes {
		totalSize += n
		if maxSize < n {
			maxSize = n
		}
	}
	rdBuf := make([]byte, maxSize)
	data := testutil.MustLoadFile(twain, totalSize)

	var buf bytes.Buffer
	wr, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	rd, err := NewReader(&buf, nil)
	if err != nil {
		t.Errorf("unexpected NewReader error: %v", err)
	}
	for i, n := range flushSizes {
		// Write and flush some portion of the test data.
		want := data[:n]
		data = data[n:]
		if _, err := wr.Write(want); err != nil {
			t.Errorf("test %d, flushSize: %d, unexpected Write error: %v", i, n, err)
		}
		if err := wr.Flush(); err != nil {
			t.Errorf("test %d, flushSize: %d, unexpected Flush error: %v", i, n, err)
		}

		// Verify that we can read all data flushed so far.
		m, err := io.ReadAtLeast(rd, rdBuf, n)
		if err != nil {
			t.Errorf("test %d, flushSize: %d, unexpected ReadAtLeast error: %v", i, n, err)
		}
		got := rdBuf[:m]
		if !bytes.Equal(got, want) {
			t.Errorf("test %d, flushSize: %d, output mismatch:\ngot  %q\nwant %q", i, n, got, want)
		}
		if buf.Len() != 0 {
			t.Errorf("test %d, flushSize: %d, unconsumed buffer data: %d bytes", i, n, buf.Len())
		}
	}
}
