// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"compress/bzip2"
	"io"
	"io/ioutil"
	"runtime"
	"testing"

	"github.com/dsnet/compress/internal/tool/bench"
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

func TestWriter(t *testing.T) {
	var loadFile = func(path string) []byte {
		buf, err := ioutil.ReadFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return buf
	}

	var vectors = []struct {
		input []byte
	}{
		{input: loadFile(binary)},
		{input: loadFile(digits)},
		{input: loadFile(huffman)},
		{input: loadFile(random)},
		{input: loadFile(repeats)},
		{input: loadFile(twain)},
		{input: loadFile(zeros)},
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

		output, err := ioutil.ReadAll(bzip2.NewReader(&buf))
		if err != nil {
			t.Errorf("test %d, read error: got %v", i, err)
		}
		if !bytes.Equal(output, v.input) {
			t.Errorf("test %d, output data mismatch", i)
		}
	}
}

func benchmarkWriter(b *testing.B, file string, level, n int) {
	b.StopTimer()
	b.SetBytes(int64(n))
	buf, err := bench.LoadFile(file, n)
	if err != nil {
		b.Fatalf("unexpected error: %v", err)
	}
	runtime.GC()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		w, err := NewWriterLevel(ioutil.Discard, level)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		w.Write(buf)
		w.Close()
	}
}

func BenchmarkEncodeDigitsSpeed1e4(b *testing.B)    { benchmarkWriter(b, digits, 1, 1e4) }
func BenchmarkEncodeDigitsSpeed1e5(b *testing.B)    { benchmarkWriter(b, digits, 1, 1e5) }
func BenchmarkEncodeDigitsSpeed1e6(b *testing.B)    { benchmarkWriter(b, digits, 1, 1e6) }
func BenchmarkEncodeDigitsDefault1e4(b *testing.B)  { benchmarkWriter(b, digits, 6, 1e4) }
func BenchmarkEncodeDigitsDefault1e5(b *testing.B)  { benchmarkWriter(b, digits, 6, 1e5) }
func BenchmarkEncodeDigitsDefault1e6(b *testing.B)  { benchmarkWriter(b, digits, 6, 1e6) }
func BenchmarkEncodeDigitsCompress1e4(b *testing.B) { benchmarkWriter(b, digits, 9, 1e4) }
func BenchmarkEncodeDigitsCompress1e5(b *testing.B) { benchmarkWriter(b, digits, 9, 1e5) }
func BenchmarkEncodeDigitsCompress1e6(b *testing.B) { benchmarkWriter(b, digits, 9, 1e6) }
func BenchmarkEncodeTwainSpeed1e4(b *testing.B)     { benchmarkWriter(b, twain, 1, 1e4) }
func BenchmarkEncodeTwainSpeed1e5(b *testing.B)     { benchmarkWriter(b, twain, 1, 1e5) }
func BenchmarkEncodeTwainSpeed1e6(b *testing.B)     { benchmarkWriter(b, twain, 1, 1e6) }
func BenchmarkEncodeTwainDefault1e4(b *testing.B)   { benchmarkWriter(b, twain, 6, 1e4) }
func BenchmarkEncodeTwainDefault1e5(b *testing.B)   { benchmarkWriter(b, twain, 6, 1e5) }
func BenchmarkEncodeTwainDefault1e6(b *testing.B)   { benchmarkWriter(b, twain, 6, 1e6) }
func BenchmarkEncodeTwainCompress1e4(b *testing.B)  { benchmarkWriter(b, twain, 9, 1e4) }
func BenchmarkEncodeTwainCompress1e5(b *testing.B)  { benchmarkWriter(b, twain, 9, 1e5) }
func BenchmarkEncodeTwainCompress1e6(b *testing.B)  { benchmarkWriter(b, twain, 9, 1e6) }
