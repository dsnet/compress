// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_bz2

package bench

import (
	"testing"
)

func TestCGoRoundTripBZ2(t *testing.T) {
	testRoundTrip(t, Encoders[FormatBZ2]["cgo"], Decoders[FormatBZ2]["cgo"])
}
