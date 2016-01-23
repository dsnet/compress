// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package flate implements the DEFLATE compressed data format,
// described in RFC 1951.
package flate

import (
	"runtime"

	"github.com/dsnet/compress"
)

const (
	maxHistSize = 1 << 15
	endBlockSym = 256
)

// Error is the wrapper type for errors specific to this library.
type Error struct{ ErrorString string }

func (e Error) Error() string  { return "flate: " + e.ErrorString }
func (e Error) CompressError() {}

// Error must also satisfy compress.Error interface.
var _ compress.Error = (*Error)(nil)

var (
	ErrCorrupt error = Error{"stream is corrupted"}
	ErrClosed  error = Error{"stream is closed"}
)

func errRecover(err *error) {
	switch ex := recover().(type) {
	case nil:
		// Do nothing.
	case runtime.Error:
		panic(ex)
	case error:
		*err = ex
	default:
		panic(ex)
	}
}
