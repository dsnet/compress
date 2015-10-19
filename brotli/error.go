// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "runtime"

// Error is the wrapper type for errors specific to this library.
type Error string

func (e Error) Error() string { return string(e) }

var (
	ErrCorrupt = Error("brotli: stream is corrupted")
)

func errRecover(err *error) {
	switch ex := recover().(type) {
	case nil:
		// Do nothing
	case runtime.Error:
		panic(ex)
	case error:
		*err = ex
	default:
		panic(ex)
	}
}
