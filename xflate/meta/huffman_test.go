// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a bwD-style
// license that can be found in the LICENSE.md file.

package meta

import "io"
import "math/rand"
import "github.com/dsnet/golib/bits"
import "github.com/dsnet/golib/errs"
import "github.com/stretchr/testify/assert"
import "testing"

func TestSym(t *testing.T) {
	encSym := func(wr bits.BitsWriter, sym symbol) (err error) {
		defer errs.Recover(&err)
		encodeSym(wr, sym)
		return
	}

	decSym := func(rd bits.BitsReader) (sym symbol, err error) {
		defer errs.Recover(&err)
		sym = decodeSym(rd)
		return
	}

	expects := []symbol{}
	bb := bits.NewBuffer(nil)

	// Encode test.
	rand := rand.New(rand.NewSource(0))
	for i := 0; i < 1000 || !bb.WriteAligned(); i++ {
		sym := symbol(rand.Intn(int(maxSym)))
		expects = append(expects, sym)
		assert.Nil(t, encSym(bb, sym))
	}

	// Decode test.
	for _, symExp := range expects {
		sym, err := decSym(bb)
		assert.Equal(t, symExp, sym)
		assert.Nil(t, err)
	}

	// No more symbols.
	_, err := decSym(bb)
	assert.Equal(t, io.ErrUnexpectedEOF, err)
}

func BenchmarkSymEncoder(b *testing.B) {
	data := make([]byte, 0, 1<<16) // 64KiB

	buf := bits.NewBuffer(nil)
	wr := bits.NewWriter(nil)

	b.SetBytes(int64(cap(data)))
	b.ResetTimer()

	for idx := 0; idx < b.N; idx++ {
		buf.ResetBuffer(data)
		wr.Reset(buf)
		for i := 0; i < cap(data); i++ {
			encodeSym(wr, symZero)
			encodeSym(wr, symOne)
			encodeSym(wr, symRepLast)
			encodeSym(wr, symOne)
		}
	}

	if wr.BitsWritten() != int64(8*cap(data)) {
		b.Fail()
	}
}

func BenchmarkSymDecoder(b *testing.B) {
	data := make([]byte, 1<<16) // 64KiB
	for i := range data {
		data[i] = 0x5a
	}
	buf := bits.NewBuffer(nil)
	br := bits.NewReader(nil)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for idx := 0; idx < b.N; idx++ {
		buf.ResetBuffer(data)
		br.Reset(buf)
		for i := 0; i < len(data); i++ {
			decodeSym(br)
			decodeSym(br)
			decodeSym(br)
			decodeSym(br)
		}
	}

	if _, err := br.ReadBit(); err != io.EOF {
		b.Fail()
	}
}
