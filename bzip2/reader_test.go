// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/testutil"
)

func TestReader(t *testing.T) {
	db := testutil.MustDecodeBitGen

	errFuncs := map[string]func(error) bool{
		"IsUnexpectedEOF": func(err error) bool { return err == io.ErrUnexpectedEOF },
		"IsCorrupted":     errors.IsCorrupted,
	}
	vectors := []struct {
		name   string // Sub-test name
		input  []byte // Test input string
		output []byte // Expected output string
		errf   string // Name of error checking callback
	}{{
		name: "EmptyString",
		errf: "IsUnexpectedEOF",
	}, {
		name:  "EmptyOutput",
		input: db(`>>> > "BZh9" H48:177245385090 H32:00000000`),
	}, {
		name: "EmptyOutput9S",
		input: db(`>>> >
			"BZh1" H48:177245385090 H32:00000000
			"BZh2" H48:177245385090 H32:00000000
			"BZh3" H48:177245385090 H32:00000000
			"BZh4" H48:177245385090 H32:00000000
			"BZh5" H48:177245385090 H32:00000000
			"BZh6" H48:177245385090 H32:00000000
			"BZh7" H48:177245385090 H32:00000000
			"BZh8" H48:177245385090 H32:00000000
			"BZh9" H48:177245385090 H32:00000000
		`),
	}, {
		name:  "InvalidStreamMagic",
		input: db(`>>> > "XX"`),
		errf:  "IsCorrupted",
	}, {
		name:  "InvalidVersion",
		input: db(`>>> > "BZX1"`),
		errf:  "IsCorrupted",
	}, {
		name:  "InvalidLevel",
		input: db(`>>> > "BZh0"`),
		errf:  "IsCorrupted",
	}, {
		name:  "InvalidBlockMagic",
		input: db(`>>> > "BZh9" H48:000000000000`),
		errf:  "IsCorrupted",
	}, {
		name: "HelloWorld",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:8e9a7706 0 H24:000003
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		output: []byte("Hello, world!"),
	}, {
		name: "HelloWorld2B",
		input: db(`>>>
			"BZh9"

			> H48:314159265359 H32:8e9a7706 0 H24:000003
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:314159265359 H32:8e9a7706 0 H24:000003
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:93ae990b
		`),
		output: db(`>>> "Hello, world!"*2`),
	}, {
		name: "HelloWorld2S",
		input: db(`>>>
			"BZh9"

			> H48:314159265359 H32:8e9a7706 0 H24:000003
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:8e9a7706

			"BZh9"

			> H48:314159265359 H32:8e9a7706 0 H24:000003
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:8e9a7706
		`),
		output: db(`>>> "Hello, world!"*2`),
	}, {
		name: "InvalidBlockChecksum",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:00000000 0 H24:000003
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:8e9a7706
		`),
		output: []byte("Hello, world!"),
		errf:   "IsCorrupted",
	}, {
		name: "InvalidStreamChecksum",
		input: db(`>>>
			"BZh9"
			> H48:314159265359 H32:8e9a7706 0 H24:000003
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111
			> H48:177245385090 H32:00000000
		`),
		output: []byte("Hello, world!"),
		errf:   "IsCorrupted",
	}, {
		// RLE1 stage with maximum repeater length.
		name: "RLE1",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:e1fac440 0 H24:000000
			< H16:8010 H16:0002 H16:8000
			> D3:2 D15:1 0
			> D5:2 0 100 11110 10100
			> D5:2 0 0 0 0
			< 0 0 01 01 111 # Pre-RLE1: "AAAA\xff"
			> H48:177245385090 H32:e1fac440
		`),
		output: db(`>>> X:41*259`),
	}, {
		// RLE1 stage with minimum repeater length.
		name: "RLE2",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:e16e6571 0 H24:000004
			< H16:0011 H16:0001 H16:0002
			> D3:2 D15:1 0
			> D5:2 0 100 11110 10100
			> D5:2 0 0 0 0
			< 0 01 01 0 111 # Pre-RLE1: "AAAA\x00"
			> H48:177245385090 H32:e16e6571
		`),
		output: db(`>>> X:41*4`),
	}, {
		// RLE1 stage with missing repeater value.
		//
		// NOTE: The C library rejects this file. Currently, our implementation
		// liberally allows this since it simplifies the implementation.
		// This may change in the future.
		name: "RLE3",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:e16e6571 0 H24:000003
			< H16:0010 H16:0002
			> D3:2 D15:1 0
			> D5:2 0 0 110
			> D5:2 0 0 110
			< 11 01 0 # Pre-RLE1: "AAAA"
			> H48:177245385090 H32:e16e6571
		`),
		output: db(`>>> X:41*4`),
	}, {
		// RLE1 stage with sub-optimal repeater usage.
		name: "RLE4",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:f59a903a 0 H24:000009
			< H16:0011 H16:0001 H16:0002
			> D3:2 D15:1 0
			> D5:1 0 10100 110 100
			> D5:2 0 0 0 0
			< 01 0 0 0 01 0 111 # Pre-RLE1: "AAAA\x00AAAA\x00"
			> H48:177245385090 H32:f59a903a
		`),
		output: db(`>>> X:41*8`),
	}, {
		// RLE1 stage with sub-optimal repeater usage.
		name: "RLE5",
		input: db(`>>>
			"BZh1"
			> H48:314159265359 H32:f59a903a 0 H24:000004
			< H16:0011 H16:0002 H16:0002
			> D3:2 D15:1 0
			> D5:3 0 110 110 10100
			> D5:2 0 0 0 0
			< 0 01 01 0 111 # Pre-RLE1: "AAAA\x01AAA"
			> H48:177245385090 H32:f59a903a
		`),
		output: db(`>>> X:41*8`),
	}, {
		// The next "stream" is only a single byte 0x30, which the Reader
		// detects as being truncated since it loads 2 bytes for the magic.
		name:  "Fuzz1",
		input: db(`>>> > "BZh8" H48:177245385090 H32:00000000 X:30`),
		errf:  "IsUnexpectedEOF", // Could be IsCorrupted
	}, {
		// Compared to Fuzz1, the next "stream" has 2 bytes 0x3030,
		// which allows the Reader to properly compare with the magic header
		// and reject the stream as invalid.
		name:  "Fuzz2",
		input: db(`>>> > "BZh8" H48:177245385090 H32:00000000 X:3030`),
		errf:  "IsCorrupted",
	}}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			rd, err := NewReader(bytes.NewReader(v.input), nil)
			if err != nil {
				t.Fatalf("unexpected NewReader error: %v", err)
			}
			output, err := ioutil.ReadAll(rd)
			if cerr := rd.Close(); cerr != nil {
				err = cerr
			}

			if !bytes.Equal(output, v.output) {
				t.Errorf("output mismatch:\ngot  %x\nwant %x", output, v.output)
			}
			if v.errf != "" && !errFuncs[v.errf](err) {
				t.Errorf("mismatching error:\ngot %v\nwant %s(err) == true", err, v.errf)
			} else if v.errf == "" && err != nil {
				t.Errorf("unexpected error: got %v", err)
			}
		})
	}
}

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
