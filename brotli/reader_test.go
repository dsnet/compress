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
		err    error  // Expected error
	}{{
		desc:   "empty string",
		input:  "",
		output: "",
		err:    io.ErrUnexpectedEOF,
	}, {
		desc:   "empty last block (padding is zero)",
		input:  "06",
		output: "",
	}, {
		desc:   "empty last block (padding is non-zero)",
		input:  "16",
		output: "",
		err:    ErrCorrupt,
	}}

	for i, v := range vectors {
		input, _ := hex.DecodeString(v.input)
		data, err := ioutil.ReadAll(NewReader(bytes.NewReader(input)))
		output := hex.EncodeToString(data)

		if err != v.err {
			t.Errorf("test %d (%q): got %v, want %v", i, v.desc, err, v.err)
		}
		if output != v.output {
			t.Errorf("test %d (%q):\ngot  %v\nwant %v", i, v.desc, output, v.output)
		}
	}
}
