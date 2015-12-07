// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"io"

	"github.com/dsnet/compress/internal/prefix"
)

type writer struct {
	codes2D [maxNumTrees][maxNumSyms]prefix.PrefixCode
	codes1D [maxNumTrees]prefix.PrefixCodes
	trees1D [maxNumTrees]prefix.Encoder
}

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
