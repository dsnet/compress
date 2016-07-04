// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

func TestReader(t *testing.T) {
	// To verify any of these inputs as valid or invalid DEFLATE streams
	// according to the C zlib library, you can use the Python wrapper library:
	//	>>> hex_string = "010100feff11"
	//	>>> import zlib
	//	>>> zlib.decompress(hex_string.decode("hex"), -15) # Negative means raw DEFLATE
	//	'\x11'
	db := testutil.MustDecodeBitGen
	dh := testutil.MustDecodeHex

	var vectors = []struct {
		desc   string // Description of the test
		input  []byte // Test input string
		output []byte // Expected output string
		inIdx  int64  // Expected input offset after reading
		outIdx int64  // Expected output offset after reading
		err    error  // Expected error
	}{{
		desc: "empty string (truncated)",
		err:  io.ErrUnexpectedEOF,
	}, {
		desc: "raw block, truncated after block header",
		input: db(`<<<
			< 0 00 0*5 # Non-last, raw block, padding
		`),
		inIdx: 1,
		err:   io.ErrUnexpectedEOF,
	}, {
		desc: "raw block, truncated in size field",
		input: db(`<<<
			< 0 00 0*5 # Non-last, raw block, padding
			< H8:0c    # RawSize: 12
		`),
		inIdx: 1,
		err:   io.ErrUnexpectedEOF,
	}, {
		desc: "raw block, truncated after size field",
		input: db(`<<<
			< 0 00 0*5 # Non-last, raw block, padding
			< H16:000c # RawSize: 12
		`),
		inIdx: 3,
		err:   io.ErrUnexpectedEOF,
	}, {
		desc: "raw block, truncated before raw data",
		input: db(`<<<
			< 0 00 0*5          # Non-last, raw block, padding
			< H16:000c H16:fff3 # RawSize: 12
		`),
		inIdx: 5,
		err:   io.ErrUnexpectedEOF,
	}, {
		desc: "raw block, truncated before raw data",
		input: db(`<<<
			< 0 00 0*5          # Non-last, raw block, padding
			< H16:000c H16:fff3 # RawSize: 12
			X:68656c6c6f        # Raw data
		`),
		output: dh("68656c6c6f"),
		inIdx:  10,
		outIdx: 5,
		err:    io.ErrUnexpectedEOF,
	}, {
		desc: "raw block, truncated before raw data",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:000c H16:fff3        # RawSize: 12
			X:68656c6c6f2c20776f726c64 # Raw data
		`),
		output: dh("68656c6c6f2c20776f726c64"),
		inIdx:  17,
		outIdx: 12,
		err:    io.ErrUnexpectedEOF,
	}, {
		desc: "raw block",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:000c H16:fff3        # RawSize: 12
			X:68656c6c6f2c20776f726c64 # Raw data

			< 1 10    # Last, fixed block
			> 0000000 # EOB marker
		`),
		output: dh("68656c6c6f2c20776f726c64"),
		inIdx:  19,
		outIdx: 12,
		err:    io.ErrUnexpectedEOF,
	}, {
		desc: "degenerate HCLenTree",
		input: db(`<<<
			< 1 10            # Last, dynamic block
			< D5:0 D5:0 D4:15 # HLit: 257, HDist: 1, HCLen: 19
			< 000*17 001 000  # HCLens: {1:1}
			> 0*256 1         # Use invalid HCLen code 1
		`),
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, empty HLitTree, empty HDistTree",
		input: db(`<<<
			< 1 10             # Last, dynamic block
			< D5:0 D5:0 D4:15  # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*15 # HCLens: {0:1}
			> 0*258            # HLits: {}, HDists: {}
		`),
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "empty HCLenTree",
		input: db(`<<<
			< 1 10            # Last, dynamic block
			< D5:0 D5:0 D4:15 # HLit: 257, HDist: 1, HCLen: 19
			< 000*19          # HCLens: {}
			> 0*258           # Use invalid HCLen code 0
		`),
		inIdx: 10,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree, use missing HDist symbol",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:0001 H16:fffe        # RawSize: 1
			X:7a                       # Raw data

			< 1 10                     # Last, dynamic block
			< D5:1 D5:0 D4:15          # HLit: 258, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1*2                # HLits: {256:1, 257:1}
			> 0                        # HDists: {}
			> 1 0                      # Use invalid HDist code 0
		`),
		output: dh("7a"),
		inIdx:  48,
		outIdx: 1,
		err:    ErrCorrupt,
	}, {
		desc: "complete HCLenTree, degenerate HLitTree, empty HDistTree",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 1 0*257                  # HLits: {0:1}, HDists: {}
			> 0*31 1                   # Use invalid HLit code 1
		`),
		output: db("<<< X:00*31"),
		inIdx:  46,
		outIdx: 31,
		err:    ErrCorrupt,
	}, {
		desc: "complete HCLenTree, degenerate HLitTree, degenerate HDistTree",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 1 0*256 1                # HLits: {0:1}, HDists: {0:1}
			> 0*31 1                   # Use invalid HLit code 1
		`),
		output: db("<<< X:00*31"),
		inIdx:  46,
		outIdx: 31,
		err:    ErrCorrupt,
	}, {
		desc: "complete HCLenTree, degenerate HLitTree, degenerate HDistTree, use missing HLit symbol",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1*2                # HLits: {256:1}, HDists: {0:1}
			> 1                        # Use invalid HLit code 1
		`),
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, too large HDistTree",
		input: db(`<<<
			< 1 10              # Last, dynamic block
			< D5:29 D5:31 D4:15 # HLit: 286, HDist: 32, HCLen: 19
			<1000011 X:05000000002004 X:00*39 X:04 # ???
		`),
		inIdx: 3,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree, excessive repeater symbol",
		input: db(`<<<
			< 1 10                           # Last, dynamic block
			< D5:29 D5:29 D4:15              # HLit: 286, HDist: 30, HCLen: 19
			< 011 000 011 001 000*13 010 000 # HCLens: {0:0,1:2,16:3,18:3}
			> 10 0*255 10 111 <D7:49 1       # Excessive repeater symbol
		`),
		inIdx: 43,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree of normal length 30",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:29 D4:15   # HLit: 257, HDist: 30, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 0 1*256 0*30       # HLits: {*:8}, HDists: {}
			> 11111111           # Compressed data (has only EOB)
		`),
		inIdx: 47,
	}, {
		desc: "complete HCLenTree, complete HLitTree, bad HDistTree",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:29 D4:15   # HLit: 257, HDist: 30, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 0 1*256 0*28 1*2   # HLits: {*:8}, HDists: {28:8, 29:8}
		`),
		inIdx: 46,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree of excessive length 31",
		input: db(`<<<
			< 1 10             # Last, dynamic block
			< D5:0 D5:30 D4:15 # HLit: 257, HDist: 31, HCLen: 19
			<0*7 X:240000000000f8 X:ff*31 X:07000000fc03 # ???
		`),
		inIdx: 3,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, over-subscribed HLitTree, empty HDistTree",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:0 D4:15    # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 1*257 0            # HLits: {*:8}
			<0*4 X:f00f          # ???
		`),
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, under-subscribed HLitTree, empty HDistTree",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:0 D4:15    # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 1*214 0*2 1*41 0   # HLits: {*:8}
			<0*4 X:f00f          # ???
		`),
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree, no EOB symbol",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:0 D4:15    # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 1*256 0*2          # HLits: {*:8}, HDists: {}
			> 00000000 11111111  # Compressed data
		`),
		output: dh("00ff"),
		inIdx:  44,
		outIdx: 2,
		err:    io.ErrUnexpectedEOF,
	}, {
		desc: "complete HCLenTree, complete HLitTree with multiple codes, empty HDistTree",
		input: db(`<<<
			< 1 10               # Last, dynamic block
			< D5:0 D5:3 D4:15    # HLit: 257, HDist: 4, HCLen: 19
			< 000*3 001*2 000*14 # HCLens: {0:1, 8:1}
			> 0 1*256 0*4        # HLits: {*:8}, HDists: {}
			> 00000000 11111111  # Compressed data
		`),
		output: dh("01"),
		inIdx:  44,
		outIdx: 1,
	}, {
		desc: "complete HCLenTree, complete HLitTree, degenerate HDistTree, use valid HDist symbol",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:0001 H16:fffe        # RawSize: 1
			X:7a                       # Raw data

			< 1 10                     # Last, dynamic block
			< D5:1 D5:0 D4:15          # HLit: 258, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1*3                # HLits: {256:1, 257:1}, HDists: {0:1}
			> 1 0*2                    # Compressed data
		`),
		output: dh("7a7a7a7a"),
		inIdx:  48,
		outIdx: 4,
	}, {
		desc: "complete HCLenTree, degenerate HLitTree, degenerate HDistTree",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1*2                # HLits: {256:1}, HDists: {0:1}
			> 0                        # Compressed data
		`),
		inIdx: 42,
	}, {
		desc: "complete HCLenTree, degenerate HLitTree, empty HDistTree",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:0 D5:0 D4:15          # HLit: 257, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1 0                # HLits: {256:1}, # HDists: {}
			> 0                        # Compressed data
		`),
		inIdx: 42,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree, spanning zero repeater symbol",
		input: db(`<<<
			< 1 10                           # Last, dynamic block
			< D5:29 D5:29 D4:15              # HLit: 286, HDist: 30, HCLen: 19
			< 011 000 011 001 000*13 010 000 # HCLens: {0:1, 1:2, 16:3, 18:3}
			> 10 0*255 10 111 <D7:48         # HLits: {0:1, 256:1}, HDists: {}
			> 1                              # Compressed data
		`),
		inIdx: 43,
	}, {
		desc: "complete HCLenTree, use last repeater on non-zero code",
		input: db(`<<<
			< 1 10           # Last, dynamic block
			< D5:0 D5:0 D4:8 # HLit: 257, HDist: 1, HClen: 12
			# HCLens: {0:2, 4:2, 16:2, 18:2}
			< 010 000 010*2 000*7 010
			# HLits: {0-14:4, 256:4}, HDists: {}
			> 01*12 10 <D2:0 11 <D7:127 11 <D7:92 01 00
			# Compressed data
			> 0000 0001 0010 1111
		`),
		output: dh("000102"),
		inIdx:  15,
		outIdx: 3,
	}, {
		desc: "complete HCLenTree, use last repeater on zero code",
		input: db(`<<<
			< 1 10           # Last, dynamic block
			< D5:0 D5:0 D4:8 # HLit: 257, HDist: 1, HClen: 12
			# HCLens: {0:2, 4:2, 16:2, 18:2}
			< 010 000 010*2 000*7 010
			# HLits: {241-256:4}, HDists: {}
			> 00 10 <D2:3 11 <D7:127 11 <D7:85 01*16 00
			# Compressed data
			> 0000 0001 0010 1111
		`),
		output: dh("f1f2f3"),
		inIdx:  16,
		outIdx: 3,
	}, {
		desc: "complete HCLenTree, use last repeater without first code",
		input: db(`<<<
			< 1 10           # Last, dynamic block
			< D5:0 D5:0 D4:8 # HLit: 257, HDist: 1, HClen: 12
			# HCLens: {0:2, 4:2, 16:2, 18:2}
			< 010 000 010*2 000*7 010
			# HLits: {???}, HDists: {???}
			> 10 <D2:3 11 <D7:127 11 <D7:86 01*16 00
			# ???
			> 0000 0001 0010 1111
		`),
		inIdx: 7,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree with length codes, complete HLitTree, empty HDistTree",
		input: db(`<<<
			< 1 10                     # Last, dynamic block
			< D5:29 D5:0 D4:15         # HLit: 286, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1 0*27 1 0*2       # HLits: {256:1, 284:1}, HDists: {}
			> 0                        # Compressed data
		`),
		inIdx: 46,
	}, {
		desc: "complete HCLenTree, complete HLitTree, degenerate HDistTree, use valid HLit symbol 284 with count 31",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:0001 H16:fffe        # RawSize: 1
			X:00                       # Raw data

			< 1 10                     # Last, dynamic block
			< D5:29 D5:0 D4:15         # HLit: 286, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1 0*27 1 0 1       # HLits: {256:1, 284:1}, HDists: {0:1}
			> 1 <D5:31 0*2             # Compressed data
		`),
		output: db("<<< X:00*259"),
		inIdx:  53,
		outIdx: 259,
	}, {
		desc: "complete HCLenTree, complete HLitTree, degenerate HDistTree, use valid HLit symbol 285",
		input: db(`<<<
			< 0 00 0*5                 # Non-last, raw block, padding
			< H16:0001 H16:fffe        # RawSize: 1
			X:00                       # Raw data

			< 1 10                     # Last, dynamic block
			< D5:29 D5:0 D4:15         # HLit: 286, HDist: 1, HCLen: 19
			< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
			> 0*256 1 0*28 1*2         # HLits: {256:1, 285:1}, HDists: {0:1}
			> 1 0*2                    # Compressed data
		`),
		output: db("<<< X:00*259"),
		inIdx:  52,
		outIdx: 259,
	}, {
		desc: "complete HCLenTree, complete HLitTree, degenerate HDistTree, use valid HLit and HDist symbols",
		input: db(`<<<
			< 0 10            # Non-last, dynamic block
			< D5:1 D5:2 D4:14 # HLit: 258, HDist: 3, HCLen: 18
			# HCLens: {0:3, 1:3, 2:2, 3:2, 18:2}
			< 000*2 010 011 000*9 010 000 010 000 011
			# HLits: {97:3, 98:3, 99:2, 256:2, 257:2}, HDists: {2:1}
			> 10 <D7:86 01 01 00 10 <D7:127 10 <D7:7 00 00 110 110 111
			# Compressed data
			> 110 111 00 10 0 01

			< 1 00 0*3          # Last, raw block, padding
			< H16:0000 H16:ffff # RawSize: 0
		`),
		output: dh("616263616263"),
		inIdx:  21,
		outIdx: 6,
	}, {
		desc: "fixed block, use reserved HLit symbol 287",
		input: db(`<<<
			< 1 01              # Last, fixed block
			> 01100000 11000111 # Use invalid symbol 287
		`),
		output: dh("30"),
		inIdx:  3,
		outIdx: 1,
		err:    ErrCorrupt,
	}, {
		desc: "fixed block, use reserved HDist symbol 30",
		input: db(`<<<
			< 1 01                   # Last, fixed block
			> 00110000 0000001 D5:30 # Use invalid HDist symbol 30
			> 0000000                # EOB marker
		`),
		output: dh("00"),
		inIdx:  3,
		outIdx: 1,
		err:    ErrCorrupt,
	}, {
		input: db(`<<<
			< 0 00 0*5                              # Non-last, raw block, padding
			< H16:8000 H16:7fff                     # RawSize: 32768
			X:0f1e2d3c4b5a69788796a5b4c3d2e1f0*2048 # Raw data

			< 1 01                     # Last, fixed block
			> 0000001 D5:29 <H13:1fff  # Length: 3, Distance: 32768
			> 11000101 D5:29 <H13:1fff # Length: 258, Distance: 32768
			> 0000000                  # EOB marker
		`),
		output: db(`<<<
			X:0f1e2d3c4b5a69788796a5b4c3d2e1f0*2048
			X:0f1e2d3c4b5a69788796a5b4c3d2e1f0*16
			X:0f1e2d3c4b
		`),
		inIdx:  32781,
		outIdx: 33029,
	}, {
		desc: "shortest fixed block",
		input: db(`<<<
			< 1 01    # Last, fixed block
			> 0000000 # EOB marker
		`),
		inIdx: 2,
	}, {
		desc: "reserved block",
		input: db(`<<<
			< 1 11 0*5 # Last, reserved block, padding
			X:deadcafe # ???
		`),
		inIdx: 1,
		err:   ErrCorrupt,
	}, {
		desc: "raw block with non-zero padding",
		input: db(`<<<
			< 1 00 10101        # Last, raw block, padding
			< H16:0001 H16:fffe # RawSize: 1
			X:11                # Raw data
		`),
		output: dh("11"),
		inIdx:  6,
		outIdx: 1,
	}, {
		desc: "shortest raw block",
		input: db(`<<<
			< 1 00 0*5          # Last, raw block, padding
			< H16:0000 H16:ffff # RawSize: 0
		`),
		inIdx: 5,
	}, {
		desc: "longest raw block",
		input: db(`<<<
			< 1 00 0*5          # Last, raw block, padding
			< H16:ffff H16:0000 # RawSize: 65535
			X:7a*65535
		`),
		output: db("<<< X:7a*65535"),
		inIdx:  65540,
		outIdx: 65535,
	}, {
		desc: "raw block with bad size",
		input: db(`<<<
			< 1 00 0*5          # Last, raw block, padding
			< H16:0001 H16:fffd # RawSize: 1
			X:11                # Raw data
		`),
		inIdx: 5,
		err:   ErrCorrupt,
	}, {
		desc: "issue 3816 - large HLitTree caused a panic",
		input: db(`<<<
			< 0 10             # Non-last, dynamic block
			< D5:31 D5:30 D4:7 # HLit: 288, HDist: 31, HCLen: 11

			# ???
			<0011011
			X:e75e1cefb3555877b656b543f46ff2d2e63d99a0858c48ebf8da83042a75c4f8
			X:0f1211b9b44b09a0be8b914c
		`),
		inIdx: 3,
		err:   ErrCorrupt,
	}, {
		desc: "issue 10426 - over-subscribed HCLenTree caused a hang",
		input: db(`<<<
			< 0 10                  # Non-last, dynamic block
			< D5:6 D5:12 D4:2       # HLit: 263, HDist: 13, HCLen: 6
			< 101 100*2 011 010 001 # HCLens: {0:3, 7:1, 8:2, 16:5, 17:4, 18:4}, invalid
			<01001 X:4d4b070000ff2e2eff2e2e2e2e2eff # ???
		`),
		inIdx: 5,
		err:   ErrCorrupt,
	}, {
		desc: "issue 11030 - empty HDistTree unexpectedly led to error",
		input: db(`<<<
			< 1 10            # Last, dynamic block
			< D5:0 D5:0 D4:14 # HLit: 257, HDist: 1, HCLen: 18
			# HCLens: {0:1, 1:4, 2:2, 16:3, 18:4}
			< 011 000 100 001 000*11 010 000 100
			# HLits: {253:2, 254:2, 255:2, 256:2}
			> 0 1111 <D7:112 1111 <D7:111 0 0 0 0 0 0 0 10 10 10 10
			# HDists: {}
			> 0
			# Compressed data
			> 11
		`),
		inIdx: 14,
	}, {
		desc: "issue 11033 - empty HDistTree unexpectedly led to error",
		input: db(`<<<
			< 1 10           # Last, dynamic block
			< D5:0 D5:0 D4:8 # HLit: 257, HDist: 1, HCLen: 12
			# HCLens: {0:2, 4:3, 5:2, 6:3, 17:3, 18:3}
			< 000 011*2 010 000*3 011 000 010 000 011
			# HLits: {...}
			> 01 110 100 101 00 00 101 111 1010000 01 110 110 01 111 0100000
			  101 00 100 01 00 00 100 01 01 111 0001000 01 111 1000000 01 110
			  010 100 00 01 110 010 01 00 00 100 110 001 100 111 0100000 01
			  111 0110000 01 00 01 111 0001010 100 110 011 01 110 110 101 00
			  101 110 011 101 110 001 101 111 0001000 101 100
			# HDists: {}
			> 00
			# Compressed data
			> 10001 0000 0000 10011 0001 0001 10000 0011 10111 111010 0100
			  0011 0100 01110 0010 111000 10010 10110 11000 111100 10101
			  111111 111001 10100 11001 11010 0010 01111 111101 111110 0101
			  11011 0101 111011 0110
		`),
		output: dh("" +
			"3130303634342068652e706870005d05355f7ed957ff084a90925d19e3ebc6d0" +
			"c6d7",
		),
		inIdx:  57,
		outIdx: 34,
	}}

	for i, v := range vectors {
		rd, err := NewReader(bytes.NewReader(v.input), nil)
		if err != nil {
			t.Errorf("test %d, unexpected NewReader error: %v", i, err)
		}
		output, err := ioutil.ReadAll(rd)
		if cerr := rd.Close(); cerr != nil {
			err = cerr
		}

		if err != v.err {
			t.Errorf("test %d, %s\nerror mismatch: got %v, want %v", i, v.desc, err, v.err)
		}
		if !bytes.Equal(output, v.output) {
			t.Errorf("test %d, %s\noutput mismatch:\ngot  %x\nwant %x", i, v.desc, output, v.output)
		}
		if rd.InputOffset != v.inIdx {
			t.Errorf("test %d, %s\ninput offset mismatch: got %d, want %d", i, v.desc, rd.InputOffset, v.inIdx)
		}
		if rd.OutputOffset != v.outIdx {
			t.Errorf("test %d, %s\noutput offset mismatch: got %d, want %d", i, v.desc, rd.OutputOffset, v.outIdx)
		}
	}
}

func TestReaderReset(t *testing.T) {
	const data = "\x00\x0c\x00\xf3\xffhello, world\x01\x00\x00\xff\xff"

	var rd Reader
	if err := rd.Close(); err != nil {
		t.Errorf("unexpected Close error: %v", err)
	}

	rd.Reset(strings.NewReader("garbage"))
	if _, err := ioutil.ReadAll(&rd); err != ErrCorrupt {
		t.Errorf("mismatching Read error: got %v, want %v", err, ErrCorrupt)
	}
	if err := rd.Close(); err != ErrCorrupt {
		t.Errorf("mismatching Close error: got %v, want %v", err, ErrCorrupt)
	}

	rd.Reset(strings.NewReader(data))
	if _, err := ioutil.ReadAll(&rd); err != nil {
		t.Errorf("unexpected Read error: %v", err)
	}
	if err := rd.Close(); err != nil {
		t.Errorf("unexpected Close error: %v", err)
	}
}

func benchmarkDecode(b *testing.B, testfile string) {
	b.StopTimer()
	b.ReportAllocs()

	input, err := ioutil.ReadFile("testdata/" + testfile)
	if err != nil {
		b.Fatal(err)
	}
	rd, err := NewReader(bytes.NewReader(input), nil)
	if err != nil {
		b.Fatal(err)
	}
	output, err := ioutil.ReadAll(rd)
	if err != nil {
		b.Fatal(err)
	}

	nb := int64(len(output))
	output = nil
	runtime.GC()

	b.SetBytes(nb)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		rd, err := NewReader(bufio.NewReader(bytes.NewReader(input)), nil)
		if err != nil {
			b.Fatalf("unexpected NewReader error: %v", err)
		}
		cnt, err := io.Copy(ioutil.Discard, rd)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if cnt != nb {
			b.Fatalf("unexpected count: got %d, want %d", cnt, nb)
		}
	}
}

func BenchmarkDecodeDigitsSpeed1e4(b *testing.B)    { benchmarkDecode(b, "digits-speed-1e4.fl") }
func BenchmarkDecodeDigitsSpeed1e5(b *testing.B)    { benchmarkDecode(b, "digits-speed-1e5.fl") }
func BenchmarkDecodeDigitsSpeed1e6(b *testing.B)    { benchmarkDecode(b, "digits-speed-1e6.fl") }
func BenchmarkDecodeDigitsDefault1e4(b *testing.B)  { benchmarkDecode(b, "digits-default-1e4.fl") }
func BenchmarkDecodeDigitsDefault1e5(b *testing.B)  { benchmarkDecode(b, "digits-default-1e5.fl") }
func BenchmarkDecodeDigitsDefault1e6(b *testing.B)  { benchmarkDecode(b, "digits-default-1e6.fl") }
func BenchmarkDecodeDigitsCompress1e4(b *testing.B) { benchmarkDecode(b, "digits-best-1e4.fl") }
func BenchmarkDecodeDigitsCompress1e5(b *testing.B) { benchmarkDecode(b, "digits-best-1e5.fl") }
func BenchmarkDecodeDigitsCompress1e6(b *testing.B) { benchmarkDecode(b, "digits-best-1e6.fl") }
func BenchmarkDecodeTwainSpeed1e4(b *testing.B)     { benchmarkDecode(b, "twain-speed-1e4.fl") }
func BenchmarkDecodeTwainSpeed1e5(b *testing.B)     { benchmarkDecode(b, "twain-speed-1e5.fl") }
func BenchmarkDecodeTwainSpeed1e6(b *testing.B)     { benchmarkDecode(b, "twain-speed-1e6.fl") }
func BenchmarkDecodeTwainDefault1e4(b *testing.B)   { benchmarkDecode(b, "twain-default-1e4.fl") }
func BenchmarkDecodeTwainDefault1e5(b *testing.B)   { benchmarkDecode(b, "twain-default-1e5.fl") }
func BenchmarkDecodeTwainDefault1e6(b *testing.B)   { benchmarkDecode(b, "twain-default-1e6.fl") }
func BenchmarkDecodeTwainCompress1e4(b *testing.B)  { benchmarkDecode(b, "twain-best-1e4.fl") }
func BenchmarkDecodeTwainCompress1e5(b *testing.B)  { benchmarkDecode(b, "twain-best-1e5.fl") }
func BenchmarkDecodeTwainCompress1e6(b *testing.B)  { benchmarkDecode(b, "twain-best-1e6.fl") }
