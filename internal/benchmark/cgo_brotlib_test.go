// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_brotli

package benchmark

import "testing"

func TestCGoRoundTripBrotli(t *testing.T) {
	testRoundTrip(t, Encoders[FormatBrotli]["cgo"], Decoders[FormatBrotli]["cgo"])
}
