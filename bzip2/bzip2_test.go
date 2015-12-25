// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

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
	var vectors = []struct {
		input []byte
	}{
		{input: nil},
		{input: testutil.MustLoadFile(binary, -1)},
		{input: testutil.MustLoadFile(digits, -1)},
		{input: testutil.MustLoadFile(huffman, -1)},
		{input: testutil.MustLoadFile(random, -1)},
		{input: testutil.MustLoadFile(repeats, -1)},
		{input: testutil.MustLoadFile(twain, -1)},
		{input: testutil.MustLoadFile(zeros, -1)},
	}

	for i, v := range vectors {
		var buf bytes.Buffer
		rd := bytes.NewReader(v.input)
		wr := NewWriter(&buf)
		cnt, err := io.Copy(wr, rd)
		if err != nil {
			t.Errorf("test %d, write error: got %v", i, err)
		}
		if cnt != int64(len(v.input)) {
			t.Errorf("test %d, write count mismatch: got %d, want %d", i, cnt, len(v.input))
		}
		if err := wr.Close(); err != nil {
			t.Errorf("test %d, close error: got %v", i, err)
		}

		output, err := ioutil.ReadAll(NewReader(&buf))
		if err != nil {
			t.Errorf("test %d, read error: got %v", i, err)
		}

		if !bytes.Equal(output, v.input) {
			t.Errorf("test %d, output data mismatch", i)
		}
	}
}
