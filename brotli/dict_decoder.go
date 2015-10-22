// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

type dictDecoder struct {
	size int    // Sliding window size
	dict []byte // Sliding window history, dynamically grown to match size
}

func (dd *dictDecoder) Init(wbits uint) {
	// Regardless of what size claims, start with a small dictionary to avoid
	// denial-of-service attacks with large memory allocation.
	dd.size = int(1<<wbits) - 16
	if dd.dict == nil {
		dd.dict = make([]byte, 4096)
	}
	dd.dict = dd.dict[:0]
}
