// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

var level int

func TestMain(m *testing.M) {
	setDefaults()
	flag.Var(&paths, "paths", "List of paths to search for test files")
	flag.Var(&globs, "globs", "List of globs to match for test files")
	flag.IntVar(&level, "level", 6, "Default compression level to use")
	flag.Parse()
	os.Exit(m.Run())
}

// TestCodecs tests that the output of each registered encoder is a valid input
// for each registered decoder. This test runs in O(n^2) where n is the number
// of registered codecs. This assumes that the number of test files and
// compression formats stays relatively constant.
func TestCodecs(t *testing.T) {
	for _, fi := range getFiles(paths, globs) {
		dd := testutil.MustLoadFile(fi.Abs)
		name := strings.Replace(fi.Rel, string(filepath.Separator), "_", -1)
		t.Run(fmt.Sprintf("File:%v", name), func(t *testing.T) {
			t.Parallel()
			testFormats(t, dd)
		})
	}
}

func testFormats(t *testing.T, dd []byte) {
	for _, ft := range formats {
		ft := ft
		t.Run(fmt.Sprintf("Format:%v", enumToFmt[ft]), func(t *testing.T) {
			t.Parallel()
			if len(encoders[ft]) == 0 || len(decoders[ft]) == 0 {
				t.Skip("no codecs available")
			}
			testEncoders(t, ft, dd)
		})
	}
}

func testEncoders(t *testing.T, ft Format, dd []byte) {
	for encName := range encoders[ft] {
		encName := encName
		t.Run(fmt.Sprintf("Encoder:%v", encName), func(t *testing.T) {
			t.Parallel()
			defer recoverPanic(t)

			be := new(bytes.Buffer)
			zw := encoders[ft][encName](be, level)
			if _, err := io.Copy(zw, bytes.NewReader(dd)); err != nil {
				t.Fatalf("unexpected Write error: %v", err)
			}
			if err := zw.Close(); err != nil {
				t.Fatalf("unexpected Close error: %v", err)
			}
			de := be.Bytes()
			testDecoders(t, ft, dd, de)
		})
	}
}

func testDecoders(t *testing.T, ft Format, dd, de []byte) {
	for decName := range decoders[ft] {
		decName := decName
		t.Run(fmt.Sprintf("Decoder:%v", decName), func(t *testing.T) {
			t.Parallel()
			defer recoverPanic(t)

			bd := new(bytes.Buffer)
			zr := decoders[ft][decName](bytes.NewReader(de))
			if _, err := io.Copy(bd, zr); err != nil {
				t.Fatalf("unexpected Read error: %v", err)
			}
			if err := zr.Close(); err != nil {
				t.Fatalf("unexpected Close error: %v", err)
			}
			if got, want, ok := testutil.Compare(bd.Bytes(), dd); !ok {
				t.Errorf("data mismatch:\ngot  %s\nwant %s", got, want)
			}
		})
	}
}

func recoverPanic(t *testing.T) {
	if ex := recover(); ex != nil {
		t.Fatalf("unexpected panic: %v", ex)
	}
}
