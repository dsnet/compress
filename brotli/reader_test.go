// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"
import "io/ioutil"
import "bytes"
import "strings"
import "encoding/hex"
import "testing"

func TestReader(t *testing.T) {
	var vectors = []struct {
		desc   string // Description of the test
		input  string // Test input string in hex
		output string // Expected output string in hex
		inIdx  int64  // Expected input offset after reading
		outIdx int64  // Expected output offset after reading
		err    error  // Expected error
	}{{
		desc: "empty string (truncated)",
		err:  io.ErrUnexpectedEOF,
	}, {
		desc:  "empty last block (WBITS is 16)",
		input: "06",
		inIdx: 1,
	}, {
		desc:  "empty last block (WBITS is 12)",
		input: "c101",
		inIdx: 2,
	}, {
		desc:  "empty last block (WBITS is 17)",
		input: "8101",
		inIdx: 2,
	}, {
		desc:  "empty last block (WBITS is 21)",
		input: "39",
		inIdx: 1,
	}, {
		desc:  "empty last block (WBITS is invalid)",
		input: "9101",
		inIdx: 1,
		err:   ErrCorrupt,
	}, {
		desc:  "empty last block (trash at the end)",
		input: "06ff",
		inIdx: 1,
	}, {
		desc:  "empty last block (padding is non-zero)",
		input: "16",
		inIdx: 1,
		err:   ErrCorrupt,
	}, {
		desc:  "empty meta data block (MLEN is 0)",
		input: "0c03",
		inIdx: 2,
	}, {
		desc:  "meta data block",
		input: "2c0648656c6c6f2c20776f726c642103",
		inIdx: 16,
	}, {
		desc:  "meta data block (truncated)",
		input: "2c06",
		inIdx: 2,
		err:   io.ErrUnexpectedEOF,
	}, {
		desc:  "meta data block (use reserved bit)",
		input: "3c0648656c6c6f2c20776f726c642103",
		inIdx: 1,
		err:   ErrCorrupt,
	}, {
		desc:  "meta data block (meta padding is non-zero)",
		input: "2c8648656c6c6f2c20776f726c642103",
		inIdx: 2,
		err:   ErrCorrupt,
	}, {
		desc:  "meta data block (non-minimal MLEN)",
		input: "4c060048656c6c6f2c20776f726c642103",
		inIdx: 3,
		err:   ErrCorrupt,
	}, {
		desc:  "meta data block (MLEN is 1<<0)",
		input: "2c00ff03",
		inIdx: 4,
	}, {
		desc:  "meta data block (MLEN is 1<<24)",
		input: "ecffff7f" + strings.Repeat("f0", 1<<24) + "03",
		inIdx: 5 + 1<<24,
	}, {
		desc:   "raw data block",
		input:  "c0001048656c6c6f2c20776f726c642103",
		output: "48656c6c6f2c20776f726c6421",
		inIdx:  17, outIdx: 13,
	}, {
		desc:  "raw data block (truncated)",
		input: "c00010",
		inIdx: 3,
		err:   io.ErrUnexpectedEOF,
	}, {
		desc:  "raw data block (raw padding is non-zero)",
		input: "c000f048656c6c6f2c20776f726c642103",
		inIdx: 3,
		err:   ErrCorrupt,
	}, {
		desc:  "raw data block (non-minimal MLEN)",
		input: "c400000148656c6c6f2c20776f726c642103",
		inIdx: 3,
		err:   ErrCorrupt,
	}, {
		desc:   "raw data block (MLEN is 1<<0)",
		input:  "0000106103",
		output: "61",
		inIdx:  4 + 1<<0, outIdx: 1 << 0,
	}, {
		desc:   "raw data block (MLEN is 1<<24)",
		input:  "f8ffff1f" + strings.Repeat("f0", 1<<24) + "03",
		output: strings.Repeat("f0", 1<<24),
		inIdx:  5 + 1<<24, outIdx: 1 << 24,
	}, {
		desc:   "compressed string: \"Hello, world! Hello, world!\"",
		input:  "1b1a00008c946ed6540dc2825426d942de6a9668ea996c961e00",
		output: "48656c6c6f2c20776f726c64212048656c6c6f2c20776f726c6421",
		inIdx:  26, outIdx: 27,
	}}

	for i, v := range vectors {
		input, _ := hex.DecodeString(v.input)
		rd := NewReader(bytes.NewReader(input))
		data, err := ioutil.ReadAll(rd)
		output := hex.EncodeToString(data)

		if err != v.err {
			t.Errorf("test %d: %s\nerror mismatch: got %v, want %v", i, v.desc, err, v.err)
			continue
		}
		if output != v.output {
			t.Errorf("test %d: %s\noutput mismatch:\ngot  %v\nwant %v", i, v.desc, output, v.output)
			continue
		}
		if rd.InputOffset != v.inIdx {
			t.Errorf("test %d: %s\ninput offset mismatch: got %d, want %d", i, v.desc, rd.InputOffset, v.inIdx)
		}
		if rd.OutputOffset != v.outIdx {
			t.Errorf("test %d: %s\noutput offset mismatch: got %d, want %d", i, v.desc, rd.OutputOffset, v.outIdx)
		}
	}
}

func TestReaderGolden(t *testing.T) {
	var vectors = []struct {
		input  string // Input filename
		output string // Output filename
	}{
		{"empty.br", "empty"},
		{"empty.00.br", "empty"},
		{"empty.01.br", "empty"},
		{"empty.02.br", "empty"},
		{"empty.03.br", "empty"},
		{"empty.04.br", "empty"},
		{"empty.05.br", "empty"},
		{"empty.06.br", "empty"},
		{"empty.07.br", "empty"},
		{"empty.08.br", "empty"},
		{"empty.09.br", "empty"},
		{"empty.10.br", "empty"},
		{"empty.11.br", "empty"},
		{"empty.12.br", "empty"},
		{"empty.13.br", "empty"},
		{"empty.14.br", "empty"},
		{"empty.15.br", "empty"},
		{"empty.16.br", "empty"},
		{"empty.17.br", "empty"},
		{"empty.18.br", "empty"},
		{"zeros.br", "zeros"},
		{"x.br", "x"},
		{"x.00.br", "x"},
		{"x.01.br", "x"},
		{"x.02.br", "x"},
		{"x.03.br", "x"},
		{"xyzzy.br", "xyzzy"},
		{"10x10y.br", "10x10y"},
		{"64x.br", "64x"},
		{"backward65536.br", "backward65536"},
		{"quickfox.br", "quickfox"},
		{"quickfox_repeated.br", "quickfox_repeated"},
		{"ukkonooa.br", "ukkonooa"},
		{"monkey.br", "monkey"},
		{"random_org_10k.bin.br", "random_org_10k.bin"},
		{"asyoulik.txt.br", "asyoulik.txt"},
		{"compressed_file.br", "compressed_file"},
		{"compressed_repeated.br", "compressed_repeated"},
		{"alice29.txt.br", "alice29.txt"},
		{"lcet10.txt.br", "lcet10.txt"},
		{"mapsdatazrh.br", "mapsdatazrh"},
		{"plrabn12.txt.br", "plrabn12.txt"},
	}

	for i, v := range vectors {
		input, err := ioutil.ReadFile("testdata/" + v.input)
		if err != nil {
			t.Errorf("test %d: %s\n%v", i, v.input, err)
			continue
		}
		output, err := ioutil.ReadFile("testdata/" + v.output)
		if err != nil {
			t.Errorf("test %d: %s\n%v", i, v.output, err)
			continue
		}

		rd := NewReader(bytes.NewReader(input))
		data, err := ioutil.ReadAll(rd)
		if err != nil {
			t.Errorf("test %d: %s\nerror mismatch: got %v, want nil", i, v.input, err)
			continue
		}
		if string(data) != string(output) {
			t.Errorf("test %d: %s\noutput mismatch:\ngot  %q\nwant %q", i, v.input, string(data), string(output))
			continue
		}
	}
}
