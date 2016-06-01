// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import (
	"bytes"
	"io"
	"testing"

	// TODO(dsnet): We should not be relying on the standard library for the
	// round-trip test.
	"compress/flate"

	"github.com/dsnet/compress/internal/testutil"
)

var testdata = []struct {
	name string
	data []byte
}{
	{"Nil", nil},
	{"Binary", testutil.MustLoadFile("../testdata/binary.bin")},
	{"Digits", testutil.MustLoadFile("../testdata/digits.txt")},
	{"Huffman", testutil.MustLoadFile("../testdata/huffman.txt")},
	{"Random", testutil.MustLoadFile("../testdata/random.bin")},
	{"Repeats", testutil.MustLoadFile("../testdata/repeats.bin")},
	{"Twain", testutil.MustLoadFile("../testdata/twain.txt")},
	{"Zeros", testutil.MustLoadFile("../testdata/zeros.bin")},
}

var levels = []struct {
	name  string
	level int
}{
	{"Huffman", flate.HuffmanOnly},
	{"Speed", flate.BestSpeed},
	{"Default", flate.DefaultCompression},
	{"Compression", flate.BestCompression},
}

var sizes = []struct {
	name string
	size int
}{
	{"1e4", 1e4},
	{"1e5", 1e5},
	{"1e6", 1e6},
}

func TestRoundTrip(t *testing.T) {
	for i, v := range testdata {
		var buf1, buf2 bytes.Buffer

		// Compress the input.
		wr, err := flate.NewWriter(&buf1, flate.DefaultCompression)
		if err != nil {
			t.Errorf("test %d, NewWriter() = (_, %v), want (_, nil)", i, err)
		}
		n, err := io.Copy(wr, bytes.NewReader(v.data))
		if n != int64(len(v.data)) || err != nil {
			t.Errorf("test %d, Copy() = (%d, %v), want (%d, nil)", i, n, err, len(v.data))
		}
		if err := wr.Close(); err != nil {
			t.Errorf("test %d, Close() = %v, want nil", i, err)
		}

		// Write a canary byte to ensure this does not get read.
		buf1.WriteByte(0x7a)

		// Decompress the output.
		rd, err := NewReader(&buf1, nil)
		if err != nil {
			t.Errorf("test %d, NewReader() = (_, %v), want (_, nil)", i, err)
		}
		n, err = io.Copy(&buf2, rd)
		if n != int64(len(v.data)) || err != nil {
			t.Errorf("test %d, Copy() = (%d, %v), want (%d, nil)", i, n, err, len(v.data))
		}
		if err := rd.Close(); err != nil {
			t.Errorf("test %d, Close() = %v, want nil", i, err)
		}
		if !bytes.Equal(buf2.Bytes(), v.data) {
			t.Errorf("test %d, output data mismatch", i)
		}

		// Read back the canary byte.
		if v, _ := buf1.ReadByte(); v != 0x7a {
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
	data := testutil.MustLoadFile("../testdata/twain.txt")
	data = testutil.ResizeData(data, totalSize)

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

func runBenchmarks(b *testing.B, f func(b *testing.B, buf []byte, lvl int)) {
	for _, td := range testdata {
		if len(td.data) == 0 {
			continue
		}
		if testing.Short() && !(td.name == "Twain" || td.name == "Digits") {
			continue
		}
		for _, tl := range levels {
			for _, ts := range sizes {
				buf := testutil.ResizeData(td.data, ts.size)
				b.Run(td.name+"/"+tl.name+"/"+ts.name, func(b *testing.B) {
					f(b, buf, tl.level)
				})
			}
		}
	}
}
