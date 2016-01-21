// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build cgo,!no_cgo_bz2

package bench

/*
#cgo LDFLAGS: -lbz2
#include "bzlib.h"

void bzStreamInit(void* zs) {
	((bz_stream*)zs)->bzalloc = NULL;
	((bz_stream*)zs)->bzfree = NULL;
	((bz_stream*)zs)->opaque = NULL;
	((bz_stream*)zs)->next_in = NULL;
	((bz_stream*)zs)->avail_in = 0;
	((bz_stream*)zs)->next_out = NULL;
	((bz_stream*)zs)->avail_out = 0;
}

int bzCompressInit(void* zs, int lvl) {
	bzStreamInit(zs);
	return BZ2_bzCompressInit((bz_stream*)zs, lvl, 0, 0);
}
int bzCompress(void* zs, int mode) { return BZ2_bzCompress((bz_stream*)zs, mode); }
int bzCompressEnd(void* zs)        { return BZ2_bzCompressEnd((bz_stream*)zs); }

int bzDecompressInit(void* zs) {
	bzStreamInit(zs);
	return BZ2_bzDecompressInit((bz_stream*)zs, 0, 0);
}
int bzDecompress(void* zs)    { return BZ2_bzDecompress((bz_stream*)zs); }
int bzDecompressEnd(void* zs) { return BZ2_bzDecompressEnd((bz_stream*)zs); }

void bzSetInput(void* zs, void* ptr, unsigned int len) {
	((bz_stream*)zs)->next_in = ptr;
	((bz_stream*)zs)->avail_in = len;
}
int bzAvailInput(void* zs) { return ((bz_stream*)zs)->avail_in; }

void bzSetOutput(void* zs, void* ptr, unsigned int len) {
	((bz_stream*)zs)->next_out = ptr;
	((bz_stream*)zs)->avail_out = len;
}
int bzAvailOutput(void* zs) { return ((bz_stream*)zs)->avail_out; }
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"
)

type bz2Err struct{ code int }

func newBZ2Err(code int) error {
	if code == C.BZ_OK {
		return nil
	}
	return &bz2Err{code}
}
func (ze *bz2Err) Error() string { return fmt.Sprintf("bzip2 error %d", ze.code) }

type bz2Stream [unsafe.Sizeof(C.bz_stream{})]byte

func (z *bz2Stream) C() unsafe.Pointer { return unsafe.Pointer(z) }

func (z *bz2Stream) setInput(buf []byte) {
	if len(buf) == 0 {
		C.bzSetInput(z.C(), nil, 0)
	} else {
		C.bzSetInput(z.C(), unsafe.Pointer(&buf[0]), C.uint(len(buf)))
	}
}
func (z *bz2Stream) availInput() int { return int(C.bzAvailInput(z.C())) }

func (z *bz2Stream) setOutput(buf []byte) {
	if len(buf) == 0 {
		C.bzSetOutput(z.C(), nil, 0)
	} else {
		C.bzSetOutput(z.C(), unsafe.Pointer(&buf[0]), C.uint(len(buf)))
	}
}
func (z *bz2Stream) availOutput() int { return int(C.bzAvailOutput(z.C())) }

type bz2Writer struct {
	wr   io.Writer
	strm *bz2Stream
	buf  [4096]byte
	mode int
	err  error
}

func newBZ2Writer(w io.Writer, lvl int) io.WriteCloser {
	zw := &bz2Writer{wr: w, strm: new(bz2Stream), mode: C.BZ_RUN}
	if ret := int(C.bzCompressInit(zw.strm.C(), C.int(lvl))); ret != C.BZ_OK {
		panic(newBZ2Err(ret))
	}
	return zw
}

func (zw *bz2Writer) Write(buf []byte) (int, error) {
	zw.strm.setInput(buf)
	for zw.err == nil {
		zw.strm.setOutput(zw.buf[:])
		if zw.strm.availOutput() > 0 {
			ret := int(C.bzCompress(zw.strm.C(), C.int(zw.mode)))
			switch ret {
			case C.BZ_RUN_OK, C.BZ_FLUSH_OK, C.BZ_FINISH_OK, C.BZ_STREAM_END:
				// Do nothing.
			default:
				zw.err = newBZ2Err(ret)
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

func (zw *bz2Writer) Close() error {
	if zw.strm == nil {
		return zw.err
	}
	zw.mode = C.BZ_FINISH
	if _, err := zw.Write(nil); zw.err == nil {
		zw.err = err
	}
	if err := newBZ2Err(int(C.bzCompressEnd(zw.strm.C()))); zw.err == nil {
		zw.err = err
	}
	zw.strm = nil
	if zw.err == nil {
		zw.err = io.ErrClosedPipe
		return nil
	}
	return zw.err
}

type bz2Reader struct {
	rd   io.Reader
	strm *bz2Stream
	buf  [4096]byte
	err  error
}

func newBZ2Reader(r io.Reader) io.ReadCloser {
	zr := &bz2Reader{rd: r, strm: new(bz2Stream)}
	if ret := int(C.bzDecompressInit(zr.strm.C())); ret != C.BZ_OK {
		panic(newBZ2Err(ret))
	}
	return zr
}

func (zr *bz2Reader) Read(buf []byte) (int, error) {
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
			ret := int(C.bzDecompress(zr.strm.C()))
			switch ret {
			case C.BZ_RUN_OK:
				// Do nothing.
			case C.BZ_STREAM_END:
				err = io.EOF
			default:
				err = newBZ2Err(ret)
			}
			if err != nil {
				zr.err = err
			}
		}
	}
	return len(buf) - zr.strm.availOutput(), zr.err
}

func (zr *bz2Reader) Close() error {
	if zr.strm == nil {
		return zr.err
	}
	if err := newBZ2Err(int(C.bzDecompressEnd(zr.strm.C()))); zr.err == nil {
		zr.err = err
	}
	zr.strm = nil
	if zr.err == io.EOF {
		return nil
	}
	return zr.err
}

func init() {
	RegisterEncoder(FormatBZ2, "cgo", newBZ2Writer)
	RegisterDecoder(FormatBZ2, "cgo", newBZ2Reader)
}
