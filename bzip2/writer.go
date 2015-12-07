// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"io"
)

type writer struct {}

func newWriter(w *io.Writer) *writer {
	bw := new(writer)
	bw.Reset(w)
	return bw
}

func (bw *writer) Write(buf []byte) (int, error) {
	return 0, nil
}

func (bw *writer) Close(buf []byte) error {
	return nil
}

func (bw *writer) Reset(w *io.Writer) {
	return
}
