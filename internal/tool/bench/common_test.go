// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bench

import (
	"bytes"
	"hash/crc32"
	"io"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

func testRoundTrip(t *testing.T, enc Encoder, dec Decoder) {
	type entry struct {
		name  string // Name of the test
		file  string // The input test file
		level int    // The size of the input
		size  int    // The compression level
	}
	var vectors []entry
	for _, f := range []string{
		"binary.bin", "digits.txt", "huffman.txt", "random.bin", "repeats.bin", "twain.txt", "zeros.bin",
	} {
		var l, s int = 6, 1e6
		vectors = append(vectors, entry{getName(f, l, s), f, l, s})
	}

	for i, v := range vectors {
		input := testutil.MustLoadFile("../../../testdata/"+v.file, v.size)
		buf := new(bytes.Buffer)
		wr := enc(buf, v.level)
		_, cpErr := io.Copy(wr, bytes.NewReader(input))
		if err := wr.Close(); err != nil {
			t.Errorf("test %d, %s: unexpected error: %v", i, v.name, err)
			continue
		}
		if cpErr != nil {
			t.Errorf("test %d, %s: unexpected error: %v", i, v.name, cpErr)
			continue
		}

		hash := crc32.NewIEEE()
		rd := dec(buf)
		cnt, cpErr := io.Copy(hash, rd)
		if err := rd.Close(); err != nil {
			t.Errorf("test %d, %s: unexpected error: %v", i, v.name, err)
			continue
		}
		if cpErr != nil {
			t.Errorf("test %d, %s: unexpected error: %v", i, v.name, cpErr)
			continue
		}

		sum := crc32.ChecksumIEEE(input)
		if int(cnt) != len(input) {
			t.Errorf("test %d, %s: mismatching count: got %d, want %d", i, v.name, cnt, len(input))
		}
		if hash.Sum32() != sum {
			t.Errorf("test %d, %s: mismatching checksum: got 0x%08x, want 0x%08x", i, v.name, hash.Sum32(), sum)
		}
	}
}
