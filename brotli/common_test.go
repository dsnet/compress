// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "testing"

// This package relies on dynamic generation of LUTs to reduce the static
// binary size. This benchmark attempts to measure the startup cost of init.
// This benchmark is not thread-safe; so do not run it in parallel with other
// tests or benchmarks!
func BenchmarkInit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		initLUTs()
	}
}
