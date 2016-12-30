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
		name: "EmptyOutput2S",
		input: db(`>>> >
			"BZh9" H48:177245385090 H32:00000000
			"BZh9" H48:177245385090 H32:00000000
		`),
	}, {
		name: "HelloWorld",
		input: db(`>>>
			"BZh9" # Stream header

			# Block header, OriginPtr:3
			> H48:314159265359 H32:8e9a7706 0 H24:000003
			# SymMap: " !,Hdelorw"
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			# NumTrees:2, NumTrees:1, Selectors: [0]
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100 # Tree0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0       # Tree1
			# Compressed data
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:8e9a7706 # Stream footer
		`),
		output: []byte("Hello, world!"),
	}, {
		name: "HelloWorld2B",
		input: db(`>>>
			"BZh9" # Stream header

			# Block header, OriginPtr:3
			> H48:314159265359 H32:8e9a7706 0 H24:000003
			# SymMap: " !,Hdelorw"
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			# NumTrees:2, NumTrees:1, Selectors: [0]
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100 # Tree0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0       # Tree1
			# Compressed data
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			# Block header, OriginPtr:3
			> H48:314159265359 H32:8e9a7706 0 H24:000003
			# SymMap: " !,Hdelorw"
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			# NumTrees:2, NumTrees:1, Selectors: [0]
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100 # Tree0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0       # Tree1
			# Compressed data
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:93ae990b # Stream footer
		`),
		output: db(`>>> "Hello, world!"*2`),
	}, {
		name: "HelloWorld2S",
		input: db(`>>>
			"BZh9" # Stream header

			# Block header, OriginPtr:3
			> H48:314159265359 H32:8e9a7706 0 H24:000003
			# SymMap: " !,Hdelorw"
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			# NumTrees:2, NumTrees:1, Selectors: [0]
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100 # Tree0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0       # Tree1
			# Compressed data
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:8e9a7706 # Stream footer

			"BZh9" # Stream header

			# Block header, OriginPtr:3
			> H48:314159265359 H32:8e9a7706 0 H24:000003
			# SymMap: " !,Hdelorw"
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			# NumTrees:2, NumTrees:1, Selectors: [0]
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100 # Tree0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0       # Tree1
			# Compressed data
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:8e9a7706 # Stream footer
		`),
		// TODO(dsnet): This should be duplicated with multistream support.
		output: db(`>>> "Hello, world!"`),
	}, {
		name: "BadBlockChecksum",
		input: db(`>>>
			"BZh9" # Stream header

			# Block header, OriginPtr:3
			> H48:314159265359 H32:00000000 0 H24:000003
			# SymMap: " !,Hdelorw"
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			# NumTrees:2, NumTrees:1, Selectors: [0]
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100 # Tree0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0       # Tree1
			# Compressed data
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:8e9a7706 # Stream footer
		`),
		output: []byte("Hello, world!"),
		errf:   "IsCorrupted",
	}, {
		name: "BadStreamChecksum",
		input: db(`>>>
			"BZh9" # Stream header

			# Block header, OriginPtr:3
			> H48:314159265359 H32:8e9a7706 0 H24:000003
			# SymMap: " !,Hdelorw"
			< H16:00d4 H16:1003 H16:0100 H16:9030 H16:0084
			# NumTrees:2, NumTrees:1, Selectors: [0]
			> D3:2 D15:1 0
			> D5:4 0 0 0 0 0 110 100 0 110 0 0 100 # Tree0
			> D5:4 0 0 0 0 0 0 0 0 110 0 0 0       # Tree1
			# Compressed data
			< 1101 000 100 000 100 0111 010 010 0011 0001 110 0111 110 1111

			> H48:177245385090 H32:00000000 # Stream footer
		`),
		output: []byte("Hello, world!"),
		errf:   "IsCorrupted",
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
				pf("%q\n", output)
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
