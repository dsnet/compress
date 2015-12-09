// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_zlib

package bench

import (
	"testing"
)

func TestCGoRoundTripZlib(t *testing.T) {
	testRoundTrip(t, Encoders[FormatFlate]["cgo"], Decoders[FormatFlate]["cgo"])
}
