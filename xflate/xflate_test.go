// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"bytes"
	"io"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

var (
	testBinary  = testutil.MustLoadFile("../testdata/binary.bin")
	testDigits  = testutil.MustLoadFile("../testdata/digits.txt")
	testHuffman = testutil.MustLoadFile("../testdata/huffman.txt")
	testRandom  = testutil.MustLoadFile("../testdata/random.bin")
	testRepeats = testutil.MustLoadFile("../testdata/repeats.bin")
	testTwain   = testutil.MustLoadFile("../testdata/twain.txt")
	testZeros   = testutil.MustLoadFile("../testdata/zeros.bin")
)

func TestRoundTrip(t *testing.T) {
	vectors := [][]byte{
		nil, testBinary, testDigits, testHuffman, testRandom, testRepeats, testTwain, testZeros,
	}

	for i, input := range vectors {
		var wb, rb bytes.Buffer

		xw, err := NewWriter(&wb, &WriterConfig{ChunkSize: 1 << 10})
		if err != nil {
			t.Errorf("test %d, unexpected error: NewWriter() = %v", i, err)
		}
		cnt, err := io.Copy(xw, bytes.NewReader(input))
		if err != nil {
			t.Errorf("test %d, unexpected error: Write() = %v", i, err)
		}
		if cnt != int64(len(input)) {
			t.Errorf("test %d, write count mismatch: got %d, want %d", i, cnt, len(input))
		}
		if err := xw.Close(); err != nil {
			t.Errorf("test %d, unexpected error: Close() = %v", i, err)
		}

		xr, err := NewReader(bytes.NewReader(wb.Bytes()), nil)
		if err != nil {
			t.Errorf("test %d, unexpected error: NewReader() = %v", i, err)
		}
		cnt, err = io.Copy(&rb, xr)
		if err != nil {
			t.Errorf("test %d, unexpected error: Read() = %v", i, err)
		}
		if cnt != int64(len(input)) {
			t.Errorf("test %d, read count mismatch: got %d, want %d", i, cnt, len(input))
		}
		if err := xr.Close(); err != nil {
			t.Errorf("test %d, unexpected error: Close() = %v", i, err)
		}

		output := rb.Bytes()
		if !bytes.Equal(output, input) {
			t.Errorf("test %d, output data mismatch", i)
		}
	}
}
