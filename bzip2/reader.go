// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"io"

	"github.com/dsnet/compress/internal/prefix"
)

type reader struct {}

func newReader(r io.Reader) *reader {
	br := new(reader)
	br.Reset(r)
	return br
}

func (br *reader) Read(buf []byte) (int, error) {
	return 0, nil
}

func (br *reader) Close(buf []byte) error {
	return nil
}

func (br *reader) Reset(r io.Reader) {
	return
}
