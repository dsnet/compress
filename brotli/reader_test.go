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
		input: "C101",
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
	}}

	for i, v := range vectors {
		input, _ := hex.DecodeString(v.input)
		rd := NewReader(bytes.NewReader(input))
		data, err := ioutil.ReadAll(rd)
		output := hex.EncodeToString(data)

		if err != v.err {
			t.Errorf("test %d: %s\nerror mismatch: got %v, want %v", i, v.desc, err, v.err)
		}
		if output != v.output {
			t.Errorf("test %d: %s\noutput mismatch:\ngot  %v\nwant %v", i, v.desc, output, v.output)
		}
		if rd.InputOffset != v.inIdx {
			t.Errorf("test %d: %s\ninput offset mismatch: got %d, want %d", i, v.desc, rd.InputOffset, v.inIdx)
		}
		if rd.OutputOffset != v.outIdx {
			t.Errorf("test %d: %s\noutput offset mismatch: got %d, want %d", i, v.desc, rd.OutputOffset, v.outIdx)
		}
	}
}
