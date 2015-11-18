// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build !no_std_lib

package benchmark

import "io"
import "io/ioutil"
import "compress/flate"
import "compress/bzip2"

func init() {
	registerEncoder(FormatFlate, "std",
		func(w io.Writer, lvl int) io.WriteCloser {
			zw, err := flate.NewWriter(w, lvl)
			if err != nil {
				panic(err)
			}
			return zw
		})
	registerDecoder(FormatFlate, "std",
		func(r io.Reader) io.ReadCloser {
			return flate.NewReader(r)
		})
	registerDecoder(FormatBZ2, "std",
		func(r io.Reader) io.ReadCloser {
			return ioutil.NopCloser(bzip2.NewReader(r))
		})
}
