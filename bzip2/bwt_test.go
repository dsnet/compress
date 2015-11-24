// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import "testing"

func TestBWT(t *testing.T) {
	var vectors = []struct {
		input  string // The input test string
		output string // Expected output string after BWT
		ptr    int    // The BWT origin pointer
	}{{
		input:  "",
		output: "",
		ptr:    -1,
	}, {
		input:  "Hello, world!",
		output: ",do!lHrellwo ",
		ptr:    3,
	}, {
		input:  "SIX.MIXED.PIXIES.SIFT.SIXTY.PIXIE.DUST.BOXES",
		output: "TEXYDST.E.IXIXIXXSSMPPS.B..E.S.EUSFXDIIOIIIT",
		ptr:    29,
	}}

	for i, v := range vectors {
		b := []byte(v.input)
		p := encodeBWT(b)
		output := string(b)
		decodeBWT(b, p)
		input := string(b)
		if input != v.input {
			t.Errorf("test %d, input mismatch:\ngot  %q\nwant %q", i, input, v.input)
		}
		if output != v.output {
			t.Errorf("test %d, output mismatch:\ngot  %q\nwant %q", i, output, v.output)
		}
		if p != v.ptr {
			t.Errorf("test %d, pointer mismatch: got %d, want %d", i, p, v.ptr)
		}
	}
}
