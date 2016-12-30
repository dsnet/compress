// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build gofuzz

package xflate_meta

import (
	"bytes"
	"io/ioutil"

	"github.com/dsnet/compress/xflate"
)

func Fuzz(data []byte) int {
	mdata, ok := decodeMeta(data)
	if ok {
		testRoundTrip(mdata)
		return 1
	} else {
		testRoundTrip(data)
		return 0
	}
}

// decodeMeta attempts to decode the metadata.
func decodeMeta(data []byte) ([]byte, bool) {
	r := bytes.NewReader(data)
	mr := xflate.NewMetaReader(r)
	b, err := ioutil.ReadAll(mr)
	return b, err == nil
}

// testRoundTrip encodes the input data and then decodes it, checking that the
// metadata was losslessly preserved.
func testRoundTrip(want []byte) {
	bb := new(bytes.Buffer)
	mw := xflate.NewMetaWriter(bb)
	n, err := mw.Write(want)
	if n != len(want) || err != nil {
		panic(err)
	}
	if err := mw.Close(); err != nil {
		panic(err)
	}

	got, ok := decodeMeta(bb.Bytes())
	if !bytes.Equal(got, want) || !ok {
		panic("mismatching bytes")
	}
}
