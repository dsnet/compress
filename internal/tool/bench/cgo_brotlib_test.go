// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_brotli

package bench

import (
	"testing"
)

func TestCGoRoundTripBrotli(t *testing.T) {
	return // TODO(dsnet): This test is flaky
	testRoundTrip(t, Encoders[FormatBrotli]["cgo"], Decoders[FormatBrotli]["cgo"])
}
