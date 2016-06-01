// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

func BenchmarkDecode(b *testing.B) {
	runBenchmarks(b, func(b *testing.B, data []byte, lvl int) {
		b.StopTimer()
		b.ReportAllocs()

		buf := new(bytes.Buffer)
		wr, _ := NewWriter(buf, &WriterConfig{Level: lvl})
		wr.Write(data)
		wr.Close()

		br := new(bytes.Reader)
		rd := new(Reader)

		b.SetBytes(int64(len(data)))
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			br.Reset(buf.Bytes())
			rd.Reset(br)

			n, err := io.Copy(ioutil.Discard, rd)
			if n != int64(len(data)) || err != nil {
				b.Fatalf("Copy() = (%d, %v), want (%d, nil)", n, err, len(data))
			}
			if err := rd.Close(); err != nil {
				b.Fatalf("Close() = %v, want nil", err)
			}
		}
	})
}
