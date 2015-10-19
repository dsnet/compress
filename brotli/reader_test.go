// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"
import "io/ioutil"
import "bytes"
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
		desc: "empty string",
		err:  io.ErrUnexpectedEOF,
	}, {
		desc:  "empty last block (padding is zero)",
		input: "06",
		inIdx: 1,
	}, {
		desc:  "empty last block (padding is zero, trash at the end)",
		input: "06ff",
		inIdx: 1,
	}, {
		desc:  "empty last block (padding is non-zero)",
		input: "16",
		inIdx: 1,
		err:   ErrCorrupt,
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
