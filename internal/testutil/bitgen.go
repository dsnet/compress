// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dsnet/compress/internal"
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
// The format consists of a series of tokens separated by unescaped white space
// of any kind. The '#' character is used for commenting. Thus, any bytes on a
// given line that appear after the '#' character is ignored.
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
// A token that begins and ends with a '"' represents literal bytes in human
// readable form. This double-quoted string is parsed in the same way that the
// Go language parses strings and is the only token that may have spaces in it.
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
func DecodeBitGen(s string) ([]byte, error) {
	t := tokenizer{s: s}

	// Check for bit-packing mode.
	var packMode byte
	if tok, ok := t.Next().(*orderToken); ok && tok.global {
		packMode = tok.order
	} else {
		return nil, errors.New("testutil: unknown stream bit-packing mode")
	}

	// Process every token in the input string.
	var bw bitBuffer
	var parseMode byte
	for tok := t.Next(); tok != nil; tok = t.Next() {
		switch tok := tok.(type) {
		case *orderToken:
			parseMode = tok.order
			if tok.global {
				return nil, errors.New("testutil: cannot set stream bit-packing mode again")
			}
		case *bitsToken:
			if tok.length > 64 || tok.value > uint64(1<<tok.length)-1 {
				return nil, errors.New("testutil: integer overflow on token")
			}
			if tok.order == '>' || (tok.order == 0 && parseMode == '>') {
				tok.value = internal.ReverseUint64N(tok.value, tok.length)
			}
			for i := 0; i < tok.repeat; i++ {
				bw.WriteBits64(tok.value, tok.length)
			}
		case *bytesToken:
			if packMode == '>' {
				// Bytes tokens should not be affected by the bit-packing
				// order. Thus, if the order is reversed, we preemptively
				// reverse the bits knowing that it will reversed back to normal
				// in the final stage.
				for i, b := range tok.value {
					tok.value[i] = internal.ReverseLUT[b]
				}
			}
			tok.value = bytes.Repeat(tok.value, tok.repeat)
			if _, err := bw.Write(tok.value); err != nil {
				return nil, err
			}
		}
	}
	if t.err != nil {
		return nil, t.err
	}

	// Apply packing bit-ordering.
	buf := bw.Bytes()
	if packMode == '>' {
		for i, b := range buf {
			buf[i] = internal.ReverseLUT[b]
		}
	}
	return buf, nil
}

type (
	orderToken struct {
		order  byte
		global bool
	}
	bitsToken struct {
		order  byte
		value  uint64
		length uint
		repeat int
	}
	bytesToken struct {
		value  []byte
		repeat int
	}
)

type tokenizer struct {
	s   string
	r   strings.Reader // Reused to avoid allocations
	err error          // Persistent error
}

func (t *tokenizer) Next() interface{} {
	if t.err != nil {
		return nil
	}

	// Skip past all whitespace and comments.
	for len(t.s) > 0 {
		if ch, n := utf8.DecodeRuneInString(t.s); unicode.IsSpace(ch) {
			t.s = t.s[n:]
		} else if t.s[0] == '#' {
			i := strings.IndexByte(t.s, '\n')
			t.s = t.s[1+(len(t.s)+i)%len(t.s):]
		} else {
			break
		}
	}

	// Handle standalone endianess markers.
	var s string
	_, err := fmt.Sscanf(t.s, "%s", &s)
	if err == nil && (s == "<" || s == "<<<" || s == ">>>" || s == ">") {
		t.s = t.s[len(s):]
		return &orderToken{s[0], s == "<<<" || s == ">>>"}
	}

	// Handle all other data tokens.
	var token interface{}
	var order byte
	var repeat *int
	if len(t.s) > 0 && (t.s[0] == '<' || t.s[0] == '>') {
		order = t.s[0]
		t.s = t.s[1:]
	}
	if len(t.s) > 0 {
		t.r.Reset(t.s)
		switch t.s[0] {
		case '0', '1':
			v := &bitsToken{order: order, repeat: 1}
			_, t.err = fmt.Fscanf(&t.r, "%b", &v.value)
			v.length = uint(len(t.s) - t.r.Len())
			token, repeat = v, &v.repeat
		case 'D':
			v := &bitsToken{order: order, repeat: 1}
			_, t.err = fmt.Fscanf(&t.r, "D%d:%d", &v.length, &v.value)
			token, repeat = v, &v.repeat
		case 'H':
			v := &bitsToken{order: order, repeat: 1}
			_, t.err = fmt.Fscanf(&t.r, "H%d:%x", &v.length, &v.value)
			token, repeat = v, &v.repeat
		case 'X':
			v := &bytesToken{repeat: 1}
			_, t.err = fmt.Fscanf(&t.r, "X:%x", &v.value)
			token, repeat = v, &v.repeat
		case '"':
			v := &bytesToken{repeat: 1}
			_, t.err = fmt.Fscanf(&t.r, "%q", &v.value)
			token, repeat = v, &v.repeat
		}
		if t.err != nil {
			return nil
		}
		t.s = t.s[len(t.s)-t.r.Len():]
	}
	if len(t.s) > 0 && t.s[0] == '*' && token != nil {
		t.r.Reset(t.s)
		if _, t.err = fmt.Fscanf(&t.r, "*%d", repeat); t.err != nil {
			return nil
		}
		t.s = t.s[len(t.s)-t.r.Len():]
	}
	if ch, n := utf8.DecodeRuneInString(t.s); !unicode.IsSpace(ch) && n > 0 {
		fmt.Sscanf(t.s, "%s", &s)
		t.err = fmt.Errorf("testutil: unknown token: %q", s)
		return nil
	}
	return token
}

// bitBuffer is a simplified implementation of prefix.Writer.
// This is implemented here to avoid a circular dependency.
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
