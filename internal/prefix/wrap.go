// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

import (
	"bytes"
	"io"
	"os"
	"strings"
)

// For some of the common Readers, we wrap and extend them to satisfy the
// compress.BufferedReader interface to improve performance.

type buffer struct {
	*bytes.Buffer
}

type bytesReader struct {
	*bytes.Reader
	pos int64
	buf []byte
	arr [512]byte
}

type stringReader struct {
	*strings.Reader
	pos int64
	buf []byte
	arr [512]byte
}

func (r *buffer) Buffered() int {
	return r.Len()
}

func (r *buffer) Peek(n int) ([]byte, error) {
	b := r.Bytes()
	if len(b) < n {
		return b, io.EOF
	}
	return b[:n], nil
}

func (r *buffer) Discard(n int) (int, error) {
	b := r.Next(n)
	if len(b) < n {
		return len(b), io.EOF
	}
	return n, nil
}

func (r *bytesReader) Buffered() int {
	if r.Len() > len(r.buf) {
		return len(r.buf)
	}
	return r.Len()
}

func (r *bytesReader) Peek(n int) ([]byte, error) {
	if n > len(r.arr) {
		return nil, io.ErrShortBuffer
	}

	// Return sub-slice of local buffer if possible.
	pos, _ := r.Seek(0, os.SEEK_CUR)
	if off := pos - r.pos; off > 0 && off < int64(len(r.buf)) {
		r.buf, r.pos = r.buf[off:], pos
	}
	if len(r.buf) >= n && r.pos == pos {
		return r.buf[:n], nil
	}

	// Fill entire local buffer, and return appropriate sub-slice.
	cnt, err := r.ReadAt(r.arr[:], pos)
	r.buf, r.pos = r.arr[:cnt], pos
	if cnt < n {
		return r.arr[:cnt], err
	}
	return r.arr[:n], nil
}

func (r *bytesReader) Discard(n int) (int, error) {
	var err error
	if n > r.Len() {
		n, err = r.Len(), io.EOF
	}
	r.Seek(int64(n), os.SEEK_CUR)
	return n, err
}

func (r *stringReader) Buffered() int {
	if r.Len() > len(r.buf) {
		return len(r.buf)
	}
	return r.Len()
}

func (r *stringReader) Peek(n int) ([]byte, error) {
	if n > len(r.arr) {
		return nil, io.ErrShortBuffer
	}

	// Return sub-slice of local buffer if possible.
	pos, _ := r.Seek(0, os.SEEK_CUR)
	if off := pos - r.pos; off > 0 && off < int64(len(r.buf)) {
		r.buf, r.pos = r.buf[off:], pos
	}
	if len(r.buf) >= n && r.pos == pos {
		return r.buf[:n], nil
	}

	// Fill entire local buffer, and return appropriate sub-slice.
	cnt, err := r.ReadAt(r.arr[:], pos)
	r.buf, r.pos = r.arr[:cnt], pos
	if cnt < n {
		return r.arr[:cnt], err
	}
	return r.arr[:n], nil
}

func (r *stringReader) Discard(n int) (int, error) {
	var err error
	if n > r.Len() {
		n, err = r.Len(), io.EOF
	}
	r.Seek(int64(n), os.SEEK_CUR)
	return n, err
}
