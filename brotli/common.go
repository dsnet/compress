// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package brotli implements the Brotli compressed data format.
package brotli

func initLUTs() {
	initContextLUTs()
	initDictLUTs()
}

func init() { initLUTs() }
