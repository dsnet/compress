// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_zlib

package bench

/*
#cgo LDFLAGS: -lz
#include "zlib.h"

void zStreamInit(void* zs) {
	((z_streamp)zs)->zalloc = Z_NULL;
	((z_streamp)zs)->zfree = Z_NULL;
	((z_streamp)zs)->opaque = Z_NULL;
	((z_streamp)zs)->next_in = Z_NULL;
	((z_streamp)zs)->avail_in = 0;
	((z_streamp)zs)->next_out = Z_NULL;
	((z_streamp)zs)->avail_out = 0;
}

int zDeflateInit(void* zs, int lvl) {
	zStreamInit(zs);
	return deflateInit2((z_streamp)zs, lvl,
		Z_DEFLATED, -MAX_WBITS, MAX_MEM_LEVEL, Z_DEFAULT_STRATEGY);
}
int zDeflate(void* zs, int mode) { return deflate((z_streamp)zs, mode); }
int zDeflateEnd(void* zs)        { return deflateEnd((z_streamp)zs); }

int zInflateInit(void* zs) {
	zStreamInit(zs);
	return inflateInit2((z_streamp)zs, -MAX_WBITS);
}
int zInflate(void* zs, int mode) { return inflate((z_streamp)zs, mode); }
int zInflateEnd(void* zs)        { return inflateEnd((z_streamp)zs); }

void zSetInput(void* zs, Bytef* ptr, uInt len) {
	((z_streamp)zs)->next_in = ptr;
	((z_streamp)zs)->avail_in = len;
}
int zAvailInput(void* zs) { return ((z_streamp)zs)->avail_in; }

void zSetOutput(void* zs, Bytef* ptr, uInt len) {
	((z_streamp)zs)->next_out = ptr;
	((z_streamp)zs)->avail_out = len;
}
int zAvailOutput(void* zs) { return ((z_streamp)zs)->avail_out; }
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"
)

type zErr struct{ code int }

func newZErr(code int) error {
	if code == C.Z_OK {
		return nil
	}
	return &zErr{code}
}
func (ze *zErr) Error() string { return fmt.Sprintf("zlib error %d", ze.code) }

type zStream [unsafe.Sizeof(C.z_stream{})]byte

func (z *zStream) C() unsafe.Pointer { return unsafe.Pointer(z) }

func (z *zStream) setInput(buf []byte) {
	if len(buf) == 0 {
		C.zSetInput(z.C(), nil, 0)
	} else {
		C.zSetInput(z.C(), (*C.Bytef)(&buf[0]), C.uInt(len(buf)))
	}
}
func (z *zStream) availInput() int { return int(C.zAvailInput(z.C())) }

func (z *zStream) setOutput(buf []byte) {
	if len(buf) == 0 {
		C.zSetOutput(z.C(), nil, 0)
	} else {
		C.zSetOutput(z.C(), (*C.Bytef)(&buf[0]), C.uInt(len(buf)))
	}
}
func (z *zStream) availOutput() int { return int(C.zAvailOutput(z.C())) }

type zWriter struct {
	wr   io.Writer
	strm *zStream
	buf  [4096]byte
	mode int
	err  error
}

func newZWriter(w io.Writer, lvl int) io.WriteCloser {
	zw := &zWriter{wr: w, strm: new(zStream), mode: C.Z_NO_FLUSH}
	if ret := int(C.zDeflateInit(zw.strm.C(), C.int(lvl))); ret != C.Z_OK {
		panic(newZErr(ret))
	}
	return zw
}

func (zw *zWriter) Write(buf []byte) (int, error) {
	zw.strm.setInput(buf)
	for zw.err == nil {
		zw.strm.setOutput(zw.buf[:])
		if zw.strm.availOutput() > 0 {
			ret := int(C.zDeflate(zw.strm.C(), C.int(zw.mode)))
			if ret == C.Z_STREAM_ERROR {
				zw.err = newZErr(ret)
			}
		}
		cnt := len(zw.buf) - zw.strm.availOutput()
		if _, err := zw.wr.Write(zw.buf[:cnt]); err != nil {
			zw.err = err
		}
		if zw.strm.availOutput() != 0 {
			break
		}
	}
	return len(buf) - zw.strm.availInput(), zw.err
}

func (zw *zWriter) Close() error {
	if zw.strm == nil {
		return zw.err
	}
	zw.mode = C.Z_FINISH
	if _, err := zw.Write(nil); zw.err == nil {
		zw.err = err
	}
	if err := newZErr(int(C.zDeflateEnd(zw.strm.C()))); zw.err == nil {
		zw.err = err
	}
	zw.strm = nil
	if zw.err == nil {
		zw.err = io.ErrClosedPipe
		return nil
	}
	return zw.err
}

type zReader struct {
	rd   io.Reader
	strm *zStream
	buf  [4096]byte
	err  error
}

func newZReader(r io.Reader) io.ReadCloser {
	zr := &zReader{rd: r, strm: new(zStream)}
	if ret := int(C.zInflateInit(zr.strm.C())); ret != C.Z_OK {
		panic(newZErr(ret))
	}
	return zr
}

func (zr *zReader) Read(buf []byte) (int, error) {
	zr.strm.setOutput(buf)
	for zr.strm.availOutput() > 0 && zr.err == nil {
		if zr.strm.availInput() == 0 {
			cnt, err := zr.rd.Read(zr.buf[:])
			if cnt > 0 && err == io.EOF {
				err = nil
			}
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				zr.err = err
			}
			zr.strm.setInput(zr.buf[:cnt])
		}
		if zr.strm.availInput() > 0 {
			var err error
			ret := int(C.zInflate(zr.strm.C(), C.Z_NO_FLUSH))
			switch ret {
			case C.Z_OK:
				// Do nothing.
			case C.Z_STREAM_END:
				err = io.EOF
			default:
				err = newZErr(ret)
			}
			if err != nil {
				zr.err = err
			}
		}
	}
	return len(buf) - zr.strm.availOutput(), zr.err
}

func (zr *zReader) Close() error {
	if zr.strm == nil {
		return zr.err
	}
	if err := newZErr(int(C.zInflateEnd(zr.strm.C()))); zr.err == nil {
		zr.err = err
	}
	zr.strm = nil
	if zr.err == io.EOF {
		return nil
	}
	return zr.err
}

func init() {
	RegisterEncoder(FormatFlate, "cgo", newZWriter)
	RegisterDecoder(FormatFlate, "cgo", newZReader)
}
