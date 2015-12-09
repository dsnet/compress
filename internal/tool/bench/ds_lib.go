// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build !no_ds_lib

package bench

import (
	"io"

	"github.com/dsnet/compress/brotli"
	"github.com/dsnet/compress/bzip2"
	"github.com/dsnet/compress/flate"
)

func init() {
	RegisterDecoder(FormatBrotli, "ds",
		func(r io.Reader) io.ReadCloser {
			return brotli.NewReader(r)
		})
	RegisterDecoder(FormatFlate, "ds",
		func(r io.Reader) io.ReadCloser {
			return flate.NewReader(r)
		})
	RegisterEncoder(FormatBZ2, "ds",
		func(w io.Writer, lvl int) io.WriteCloser {
			zw, err := bzip2.NewWriterLevel(w, lvl)
			if err != nil {
				panic(err)
			}
			return zw
		})
	RegisterDecoder(FormatBZ2, "ds",
		func(r io.Reader) io.ReadCloser {
			return bzip2.NewReader(r)
		})
}
