// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_brotli

package bench

import (
	"io"

	"gopkg.in/kothar/brotli-go.v0/dec"
	"gopkg.in/kothar/brotli-go.v0/enc"
)

func init() {
	RegisterEncoder(FormatBrotli, "cgo",
		func(w io.Writer, lvl int) io.WriteCloser {
			c := enc.NewBrotliParams()
			c.SetQuality(lvl)
			return enc.NewBrotliWriter(c, w)
		})
	RegisterDecoder(FormatBrotli, "cgo",
		func(r io.Reader) io.ReadCloser {
			return dec.NewBrotliReaderSize(r, 4096)
		})
}
