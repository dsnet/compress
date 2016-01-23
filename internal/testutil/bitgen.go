// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package testutil

import (
	"bytes"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/dsnet/compress/internal"
)

var (
	reBin = regexp.MustCompile("^[01]{1,64}$")
	reDec = regexp.MustCompile("^D[0-9]+:[0-9]+$")
	reHex = regexp.MustCompile("^H[0-9]+:[0-9a-fA-F]{1,16}$")
	reRaw = regexp.MustCompile("^X:[0-9a-fA-F]+$")
	reQnt = regexp.MustCompile("[*][0-9]+$")
)

// DecodeBitGen decodes a BitGen formatted string.
//
// The BitGen format allows bit-streams to be generated from a series of tokens
// describing bits in the resulting string. The format is designed for testing
// purposes by aiding a human in the manual scripting of compression stream
// from individual bit-strings. It is designed to be relatively succinct, but
// allow the user to have control over the bit-order and also to allow the
// presence of comments to encode authorial intent.
//
// The format consists of a series of tokens separated by white space of any
// kind. The '#' character is used for commenting. Thus, any bytes on a given
// line that appear after the '#' character is ignored.
//
// The first valid token must either be a "<<<" (little-endian) or a ">>>"
// (big-endian). This determines whether the preceding bits in the stream are
// packed starting with the least-significant bits of a byte (little-endian) or
// packed starting with the most-significant bits of a byte (big-endian).
// Formats like DEFLATE and Brotli use little-endian, while BZip2 uses a
// big-endian bit-packing mode. This token appears exactly once at the start.
//
// A token of the form "<" (little-endian) or ">" (big-endian) determines the
// current bit-parsing mode, which alters the way subsequent tokens are
// processed. The format defaults to using a little-endian bit-parsing mode.
//
// A token of the pattern "[01]{1,64}" forms a bit-string (e.g. 11010).
// If the current bit-parsing mode is little-endian, then the right-most bits of
// the bit-string are written first to the resulting bit-stream. Likewise, if
// the bit-parsing mode is big-endian, then the left-most bits of the bit-string
// are written first to the resulting bit-stream.
//
// A token of the pattern "D[0-9]+:[0-9]+" or "H[0-9]+:[0-9a-fA-F]{1,16}"
// represents either a decimal value or a hexadecimal value, respectively.
// This numeric value is converted to the unsigned binary representation and
// used as the bit-string to write. The first number indicates the bit-length
// of the bit-string and must be between 0 and 64 bits. The second number
// represents the numeric value. The bit-length must be long enough to contain
// the resulting binary value. If the current bit-parsing mode is little-endian,
// then the least-significant bits of this binary number are written first to
// the resulting bit-stream. Likewise, the opposite holds for big-endian mode.
//
// A token that is of the pattern "X:[0-9a-fA-F]+" represents literal bytes in
// hexadecimal format that should be written to the resulting bit-stream.
// This token is affected by neither the bit-packing nor the bit-parsing modes.
// However, it may only be used when the bit-stream is already byte-aligned.
//
// A token decorator of "<" (little-endian) or ">" (big-endian) may begin
// any binary token or decimal token. This will affect the bit-parsing mode
// for that token only. It will not set the overall global mode. That still
// needs to be done by standalone "<" and ">" tokens. This decorator has no
// effect if applied to the literal bytes token.
//
// A token decorator of the pattern "[*][0-9]+" may trail any token. This is
// a quantifier decorator which indicates that the current token is to be
// repeated some number of times. It is used to quickly replicate data and
// allows the format to quickly generate large quantities of data.
//
// If the total bit-stream does not end on a byte-aligned edge, then the stream
// will automatically be padded up to the nearest byte with 0 bits.
//
// Example BitGen file:
//	<<< # DEFLATE uses LE bit-packing order
//
//	< 0 00 0*5                 # Non-last, raw block, padding
//	< H16:0004 H16:fffb        # RawSize: 4
//	X:deadcafe                 # Raw data
//
//	< 1 10                     # Last, dynamic block
//	< D5:1 D5:0 D4:15          # HLit: 258, HDist: 1, HCLen: 19
//	< 000*3 001 000*13 001 000 # HCLens: {0:1, 1:1}
//	> 0*256 1*2                # HLits: {256:1, 257:1}
//	> 0                        # HDists: {}
//	> 1 0                      # Use invalid HDist code 0
//
// Generated output stream (in hexadecimal):
//	"000400fbffdeadcafe0de0010400000000100000000000000000000000000000" +
//	"0000000000000000000000000000000000002c"
func DecodeBitGen(str string) ([]byte, error) {
	// Tokenize the input string by removing comments and superfluous spaces.
	var toks []string
	for _, s := range strings.Split(str, "\n") {
		if i := strings.IndexByte(s, '#'); i >= 0 {
			s = s[:i]
		}
		for _, t := range strings.Split(s, " ") {
			t = strings.TrimSpace(t)
			if len(t) > 0 {
				toks = append(toks, t)
			}
		}
	}
	if len(toks) == 0 {
		toks = append(toks, "")
	}

	// Check for bit-packing mode.
	var packMode bool // Bit-parsing mode: false is LE, true is BE
	switch toks[0] {
	case "<<<":
		packMode = false
	case ">>>":
		packMode = true
	default:
		return nil, errors.New("testutil: unknown stream bit-packing mode")
	}
	toks = toks[1:]

	var bw bitBuffer
	var parseMode bool // Bit-parsing mode: false is LE, true is BE
	for _, t := range toks {
		// Check for local and global bit-parsing mode modifiers.
		pm := parseMode
		if t[0] == '<' || t[0] == '>' {
			pm = bool(t[0] == '>')
			t = t[1:]
			if len(t) == 0 {
				parseMode = pm // This is a global modifier, so remember it
				continue
			}
		}

		// Check for quantifier decorators.
		rep := 1
		if reQnt.MatchString(t) {
			i := strings.LastIndexByte(t, '*')
			tt, tn := t[:i], t[i+1:]
			n, err := strconv.Atoi(tn)
			if err != nil {
				return nil, errors.New("testutil: invalid quantified token: " + t)
			}
			t, rep = tt, n
		}

		switch {
		case reBin.MatchString(t):
			// Handle binary tokens.
			var v uint64
			for _, b := range t {
				v <<= 1
				v |= uint64(b - '0')
			}

			if pm {
				v = internal.ReverseUint64N(v, uint(len(t)))
			}
			for i := 0; i < rep; i++ {
				bw.WriteBits64(v, uint(len(t)))
			}
		case reDec.MatchString(t) || reHex.MatchString(t):
			// Handle decimal and hexadecimal tokens.
			i := strings.IndexByte(t, ':')
			tb, tn, tv := t[0], t[1:i], t[i+1:]

			base := 10
			if tb == 'H' {
				base = 16
			}

			n, err1 := strconv.Atoi(tn)
			v, err2 := strconv.ParseUint(tv, base, 64)
			if err1 != nil || err2 != nil || n > 64 {
				return nil, errors.New("testutil: invalid numeric token: " + t)
			}
			if n < 64 && v&((1<<uint(n))-1) != v {
				return nil, errors.New("testutil: integer overflow on token: " + t)
			}

			if pm {
				v = internal.ReverseUint64N(v, uint(n))
			}
			for i := 0; i < rep; i++ {
				bw.WriteBits64(v, uint(n))
			}
		case reRaw.MatchString(t):
			// Handle hexadecimal tokens.
			tx := t[2:]
			b, err := hex.DecodeString(tx)
			if err != nil {
				return nil, errors.New("testutil: invalid raw bytes token: " + t)
			}
			b = bytes.Repeat(b, rep)
			if _, err := bw.Write(b); err != nil {
				return nil, err
			}
		default:
			// Handle invalid tokens.
			return nil, errors.New("testutil: invalid token: " + t)
		}
	}

	// Apply packing bit-ordering.
	buf := bw.Bytes()
	if packMode {
		for i, b := range buf {
			buf[i] = internal.ReverseLUT[b]
		}
	}
	return buf, nil
}

// bitBuffer is a simplified and minified implementation of prefix.Writer.
// This is implemented here to avoid a diamond dependency.
type bitBuffer struct {
	b []byte
	m byte
}

func (b *bitBuffer) Write(buf []byte) (int, error) {
	if b.m != 0x00 {
		return 0, errors.New("testutil: unaligned write")
	}
	b.b = append(b.b, buf...)
	return len(buf), nil
}

func (b *bitBuffer) WriteBits64(v uint64, n uint) {
	for i := uint(0); i < n; i++ {
		if b.m == 0x00 {
			b.m = 0x01
			b.b = append(b.b, 0x00)
		}
		if v&(1<<i) != 0 {
			b.b[len(b.b)-1] |= b.m
		}
		b.m <<= 1
	}
}

func (b *bitBuffer) Bytes() []byte {
	return b.b
}
