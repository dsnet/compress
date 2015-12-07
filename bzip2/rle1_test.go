// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestRunLengthEncoder(t *testing.T) {
	var vectors = []struct {
		size   int
		input  string
		output string
		done   bool
	}{{
		size:   0,
		input:  "",
		output: "",
	}, {
		size:   6,
		input:  "abc",
		output: "abc",
	}, {
		size:   6,
		input:  "abcccc",
		output: "abccc",
		done:   true,
	}, {
		size:   7,
		input:  "abcccc",
		output: "abcccc\x00",
	}, {
		size:   14,
		input:  "aaaabbbbcccc",
		output: "aaaa\x00bbbb\x00ccc",
		done:   true,
	}, {
		size:   15,
		input:  "aaaabbbbcccc",
		output: "aaaa\x00bbbb\x00cccc\x00",
	}, {
		size:   16,
		input:  strings.Repeat("a", 4),
		output: "aaaa\x00",
	}, {
		size:   16,
		input:  strings.Repeat("a", 255),
		output: "aaaa\xfb",
	}, {
		size:   16,
		input:  strings.Repeat("a", 256),
		output: "aaaa\xfba",
	}, {
		size:   16,
		input:  strings.Repeat("a", 259),
		output: "aaaa\xfbaaaa\x00",
	}, {
		size:   16,
		input:  strings.Repeat("a", 500),
		output: "aaaa\xfbaaaa\xf1",
	}, {
		size:   64,
		input:  "aaabbbcccddddddeeefgghiiijkllmmmmmmmmnnoo",
		output: "aaabbbcccdddd\x02eeefgghiiijkllmmmm\x04nnoo",
	}}

	buf := make([]byte, 3)
	for i, v := range vectors {
		rle := new(runLengthEncoding)
		rle.Init(make([]byte, v.size))
		_, err := io.CopyBuffer(rle, strings.NewReader(v.input), buf)
		output := string(rle.Bytes())

		if output != v.output {
			t.Errorf("test %d, output mismatch:\ngot  %q\nwant %q", i, output, v.output)
		}
		if done := err == rleDone; done != v.done {
			t.Errorf("test %d, done mismatch: got %v want %v", i, done, v.done)
		}
	}
}

func TestRunLengthDecoder(t *testing.T) {
	var vectors = []struct {
		input  string
		output string
	}{{
		input:  "",
		output: "",
	}, {
		input:  "abc",
		output: "abc",
	}, {
		input:  "aaaa",
		output: "aaaa",
	}, {
		input:  "baaaa\x00aaaa",
		output: "baaaaaaaa",
	}, {
		input:  "abcccc\x00",
		output: "abcccc",
	}, {
		input:  "aaaa\x00bbbb\x00ccc",
		output: "aaaabbbbccc",
	}, {
		input:  "aaaa\x00bbbb\x00cccc\x00",
		output: "aaaabbbbcccc",
	}, {
		input:  "aaaa\x00aaaa\x00aaaa\x00",
		output: "aaaaaaaaaaaa",
	}, {
		input:  "aaaa\xffaaaa\xffaaaa\xff",
		output: strings.Repeat("a", 259*3),
	}, {
		input:  "bbbaaaa\xffaaaa\xffaaaa\xff",
		output: "bbb" + strings.Repeat("a", 259*3),
	}, {
		input:  "aaaa\x00",
		output: strings.Repeat("a", 4),
	}, {
		input:  "aaaa\xfb",
		output: strings.Repeat("a", 255),
	}, {
		input:  "aaaa\xfba",
		output: strings.Repeat("a", 256),
	}, {
		input:  "aaaa\xfbaaaa\x00",
		output: strings.Repeat("a", 259),
	}, {
		input:  "aaaa\xfbaaaa\xf1",
		output: strings.Repeat("a", 500),
	}, {
		input:  "aaabbbcccdddd\x02eeefgghiiijkllmmmm\x04nnoo",
		output: "aaabbbcccddddddeeefgghiiijkllmmmmmmmmnnoo",
	}}

	buf := make([]byte, 3)
	for i, v := range vectors {
		rle := new(runLengthEncoding)
		wr := new(bytes.Buffer)
		rle.Init([]byte(v.input))
		io.CopyBuffer(wr, rle, buf)
		output := wr.String()

		if output != v.output {
			t.Errorf("test %d, output mismatch:\ngot  %q\nwant %q", i, output, v.output)
		}
	}
}
