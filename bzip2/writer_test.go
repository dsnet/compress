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

func benchmarkEncode(b *testing.B, file string, level, n int) {
	b.StopTimer()
	b.SetBytes(int64(n))
	buf := testutil.MustLoadFile(file, n)
	runtime.GC()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		w, err := NewWriter(ioutil.Discard, &WriterConfig{Level: level})
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if _, err := io.Copy(w, bytes.NewBuffer(buf)); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if err := w.Close(); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkEncodeDigitsSpeed1e4(b *testing.B)    { benchmarkEncode(b, digits, 1, 1e4) }
func BenchmarkEncodeDigitsSpeed1e5(b *testing.B)    { benchmarkEncode(b, digits, 1, 1e5) }
func BenchmarkEncodeDigitsSpeed1e6(b *testing.B)    { benchmarkEncode(b, digits, 1, 1e6) }
func BenchmarkEncodeDigitsDefault1e4(b *testing.B)  { benchmarkEncode(b, digits, 6, 1e4) }
func BenchmarkEncodeDigitsDefault1e5(b *testing.B)  { benchmarkEncode(b, digits, 6, 1e5) }
func BenchmarkEncodeDigitsDefault1e6(b *testing.B)  { benchmarkEncode(b, digits, 6, 1e6) }
func BenchmarkEncodeDigitsCompress1e4(b *testing.B) { benchmarkEncode(b, digits, 9, 1e4) }
func BenchmarkEncodeDigitsCompress1e5(b *testing.B) { benchmarkEncode(b, digits, 9, 1e5) }
func BenchmarkEncodeDigitsCompress1e6(b *testing.B) { benchmarkEncode(b, digits, 9, 1e6) }
func BenchmarkEncodeTwainSpeed1e4(b *testing.B)     { benchmarkEncode(b, twain, 1, 1e4) }
func BenchmarkEncodeTwainSpeed1e5(b *testing.B)     { benchmarkEncode(b, twain, 1, 1e5) }
func BenchmarkEncodeTwainSpeed1e6(b *testing.B)     { benchmarkEncode(b, twain, 1, 1e6) }
func BenchmarkEncodeTwainDefault1e4(b *testing.B)   { benchmarkEncode(b, twain, 6, 1e4) }
func BenchmarkEncodeTwainDefault1e5(b *testing.B)   { benchmarkEncode(b, twain, 6, 1e5) }
func BenchmarkEncodeTwainDefault1e6(b *testing.B)   { benchmarkEncode(b, twain, 6, 1e6) }
func BenchmarkEncodeTwainCompress1e4(b *testing.B)  { benchmarkEncode(b, twain, 9, 1e4) }
func BenchmarkEncodeTwainCompress1e5(b *testing.B)  { benchmarkEncode(b, twain, 9, 1e5) }
func BenchmarkEncodeTwainCompress1e6(b *testing.B)  { benchmarkEncode(b, twain, 9, 1e6) }
