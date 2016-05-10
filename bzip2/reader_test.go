// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"io"
	"io/ioutil"
	"runtime"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

func benchmarkDecode(b *testing.B, file string, level, n int) {
	b.StopTimer()
	b.SetBytes(int64(n))
	buf := testutil.MustLoadFile(file, n)
	w := new(bytes.Buffer)
	wr, err := NewWriter(w, &WriterConfig{Level: level})
	if err != nil {
		b.Fatalf("unexpected error: %v", err)
	}
	if _, err := wr.Write(buf); err != nil {
		b.Fatalf("unexpected error: %v", err)
	}
	if err := wr.Close(); err != nil {
		b.Fatalf("unexpected error: %v", err)
	}
	runtime.GC()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r, err := NewReader(bytes.NewBuffer(w.Bytes()), nil)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if _, err := io.Copy(ioutil.Discard, r); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if err := r.Close(); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkDecodeDigitsSpeed1e4(b *testing.B)    { benchmarkDecode(b, digits, 1, 1e4) }
func BenchmarkDecodeDigitsSpeed1e5(b *testing.B)    { benchmarkDecode(b, digits, 1, 1e5) }
func BenchmarkDecodeDigitsSpeed1e6(b *testing.B)    { benchmarkDecode(b, digits, 1, 1e6) }
func BenchmarkDecodeDigitsDefault1e4(b *testing.B)  { benchmarkDecode(b, digits, 6, 1e4) }
func BenchmarkDecodeDigitsDefault1e5(b *testing.B)  { benchmarkDecode(b, digits, 6, 1e5) }
func BenchmarkDecodeDigitsDefault1e6(b *testing.B)  { benchmarkDecode(b, digits, 6, 1e6) }
func BenchmarkDecodeDigitsCompress1e4(b *testing.B) { benchmarkDecode(b, digits, 9, 1e4) }
func BenchmarkDecodeDigitsCompress1e5(b *testing.B) { benchmarkDecode(b, digits, 9, 1e5) }
func BenchmarkDecodeDigitsCompress1e6(b *testing.B) { benchmarkDecode(b, digits, 9, 1e6) }
func BenchmarkDecodeTwainSpeed1e4(b *testing.B)     { benchmarkDecode(b, twain, 1, 1e4) }
func BenchmarkDecodeTwainSpeed1e5(b *testing.B)     { benchmarkDecode(b, twain, 1, 1e5) }
func BenchmarkDecodeTwainSpeed1e6(b *testing.B)     { benchmarkDecode(b, twain, 1, 1e6) }
func BenchmarkDecodeTwainDefault1e4(b *testing.B)   { benchmarkDecode(b, twain, 6, 1e4) }
func BenchmarkDecodeTwainDefault1e5(b *testing.B)   { benchmarkDecode(b, twain, 6, 1e5) }
func BenchmarkDecodeTwainDefault1e6(b *testing.B)   { benchmarkDecode(b, twain, 6, 1e6) }
func BenchmarkDecodeTwainCompress1e4(b *testing.B)  { benchmarkDecode(b, twain, 9, 1e4) }
func BenchmarkDecodeTwainCompress1e5(b *testing.B)  { benchmarkDecode(b, twain, 9, 1e5) }
func BenchmarkDecodeTwainCompress1e6(b *testing.B)  { benchmarkDecode(b, twain, 9, 1e6) }
