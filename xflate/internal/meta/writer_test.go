// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import (
	"bytes"
	"io"
	"math/rand"
	"testing"

	"github.com/dsnet/compress/internal/errors"
	"github.com/dsnet/compress/internal/testutil"
)

// TestWriter tests that the encoded output matches the expected output exactly.
// A failure here does not necessarily mean that the encoder is wrong since
// there are many possible representations. Before changing the test vectors to
// make a test pass, one must verify the new output is correct.
func TestWriter(t *testing.T) {
	dh := testutil.MustDecodeHex

	errFuncs := map[string]func(error) bool{
		"IsInvalid": errors.IsInvalid,
	}
	vectors := []struct {
		desc   string    // Description of the text
		input  []byte    // Test input string
		output []byte    // Expected output string
		final  FinalMode // Input final mode
		errf   string    // Name of error checking callback
	}{{
		desc:   "empty meta block (FinalNil)",
		input:  dh(""),
		output: dh("1c408705000000f2ffc7ede0"),
		final:  FinalNil,
	}, {
		desc:   "empty meta block (FinalMeta)",
		input:  dh(""),
		output: dh("1c408705000000d2ff1fb7e1"),
		final:  FinalMeta,
	}, {
		desc:   "input string 'a'",
		input:  dh("61"),
		output: dh("340087050000483232eaff4bdb0bf0"),
		final:  FinalMeta,
	}, {
		desc:   "input string 'ab'",
		input:  dh("6162"),
		output: dh("04008705000048848c22d4ff6fb6f3"),
		final:  FinalMeta,
	}, {
		desc:   "input string 'abc'",
		input:  dh("616263"),
		output: dh("04c086050020296414a114eaffebda7bfb"),
		final:  FinalMeta,
	}, {
		desc:   "input string 'Hello, world!' with FinalNil",
		input:  dh("48656c6c6f2c20776f726c6421"),
		output: dh("3c8086058090322289422994d25028d951a9341451a114a264747e7b02fc"),
		final:  FinalNil,
	}, {
		desc:   "input string 'Hello, world!' with FinalMeta",
		input:  dh("48656c6c6f2c20776f726c6421"),
		output: dh("348086058024654412855228a5a150b2a3526928a2422944c9e8fdf602fc"),
		final:  FinalMeta,
	}, {
		desc:   "input string 'Hello, world!' with FinalStream",
		input:  dh("48656c6c6f2c20776f726c6421"),
		output: dh("358086058024654412855228a5a150b2a3526928a2422944c9e8fdf602fc"),
		final:  FinalStream,
	}, {
		desc:   "input hex-string '00'*4",
		input:  dh("00000000"),
		output: dh("3440870500000012faffe026e0"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string '00'*8",
		input:  dh("0000000000000000"),
		output: dh("1c40870500000092d1ffff36e1"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string '00'*16",
		input:  dh("00000000000000000000000000000000"),
		output: dh("1c40870500000092d5fff736e1"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string 'ff'*4",
		input:  dh("ffffffff"),
		output: dh("2c40870500000052f4ffc32de0"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string 'ff'*8",
		input:  dh("ffffffffffffffff"),
		output: dh("2440870500000052e8ff835be0"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string 'ff'*16",
		input:  dh("ffffffffffffffffffffffffffffffff"),
		output: dh("0c4087050000005246ffffdbe2"),
		final:  FinalMeta,
	}, {
		desc:   "the random hex-string '911fe47084a4668b'",
		input:  dh("911fe47084a4668b"),
		output: dh("2480860580642444d38acaa890119114a584febfa0bdf7de03fc"),
		final:  FinalMeta,
	}, {
		desc:   "the random hex-string 'de9fa94cb16f40fc'",
		input:  dh("de9fa94cb16f40fc"),
		output: dh("0c808605801492915d94a428a9c88ab6eaff27da7bef5dfc"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string '55'*22",
		input:  dh("55555555555555555555555555555555555555555555"),
		output: dh("0540860512a52449922449922449922449922449922449922449922449922449922449922449d237edbdf79efe"),
		final:  FinalStream,
	}, {
		desc:   "input hex-string '55'*23",
		input:  dh("5555555555555555555555555555555555555555555555"),
		output: dh("04408605924a499224499224499224499224499224499224499224499224499224499224499224493aa6bdf7defe"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string '55'*24",
		input:  dh("555555555555555555555555555555555555555555555555"),
		output: dh("3c408605322b4992244992244992244992244992244992244992244992244992244992244992244992f4487bef3d01fe"),
		final:  FinalNil,
	}, {
		desc:   "input hex-string '55'*25",
		input:  dh("55555555555555555555555555555555555555555555555555"),
		output: dh("3540860592a824499224499224499224499224499224499224499224499224499224499224499224499224fdd1de7b02fe"),
		final:  FinalStream,
	}, {
		desc:   "input hex-string '55'*26",
		input:  dh("5555555555555555555555555555555555555555555555555555"),
		output: dh("2c40860512a92449922449922449922449922449922449922449922449922449922449922449922449922449d217edbd03fe"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string '55'*27",
		input:  dh("555555555555555555555555555555555555555555555555555555"),
		output: dh("1c40860542a924499224499224499224499224499224499224499224499224499224499224499224499224499224fdd0de03fe"),
		final:  FinalNil,
	}, {
		desc:   "input hex-string '55'*28",
		input:  dh("55555555555555555555555555555555555555555555555555555555"),
		output: dh("2d408605121a9224499224499224499224499224499224499224499224499224499224499224499224499224499224e983f604fe"),
		final:  FinalStream,
	}, {
		desc:   "input hex-string '55'*29",
		input:  dh("5555555555555555555555555555555555555555555555555555555555"),
		output: dh("2c4086059234244992244992244992244992244992244992244992244992244992244992244992244992244992244992241dd006fe"),
		final:  FinalMeta,
	}, {
		desc:   "input hex-string '55'*30",
		input:  dh("555555555555555555555555555555555555555555555555555555555555"),
		output: dh("34408605325a9224499224499224499224499224499224499224499224499224499224499224499224499224499224499224c96c00fe"),
		final:  FinalNil,
	}, {
		desc:  "input hex-string '55'*31",
		input: dh("55555555555555555555555555555555555555555555555555555555555555"),
		final: FinalStream,
		errf:  "IsInvalid",
	}, {
		desc:  "input hex-string '55'*32",
		input: dh("5555555555555555555555555555555555555555555555555555555555555555"),
		final: FinalMeta,
		errf:  "IsInvalid",
	}, {
		desc:   "input hex-string '73de76bebcf69d5fed3fb3cee87bacfd7de876facffedf'",
		input:  dh("73de76bebcf69d5fed3fb3cee87bacfd7de876facffedf"),
		output: dh("14808605806888421911a1ac9491c80a6526914d51241495ac8c8a447656438850b2297aa0d72afc"),
		final:  FinalNil,
	}, {
		desc:  "input hex-string '73de76bebcf69d5fed3fb3cee87bacfd7de876facffede'",
		input: dh("73de76bebcf69d5fed3fb3cee87bacfd7de876facffede"),
		final: FinalStream,
		errf:  "IsInvalid",
	}, {
		desc:   "input hex-string 'def773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9'",
		input:  dh("def773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9"),
		output: dh("3c808605809496919559c80c49240d252be98f9095cc6c6584105995112259654b2f444676dd50a4c85601fc"),
		final:  FinalMeta,
	}, {
		desc:  "input hex-string 'dff773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9'",
		input: dh("dff773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9"),
		final: FinalMeta,
		errf:  "IsInvalid",
	}}

	for i, v := range vectors {
		var b bytes.Buffer
		mw := NewWriter(&b)
		mw.bufCnt = copy(mw.buf[:], v.input)
		for _, b := range v.input {
			b0s, b1s := numBits(b)
			mw.buf0s, mw.buf1s = b0s+mw.buf0s, b1s+mw.buf1s
		}
		err := mw.encodeBlock(v.final)
		output := b.Bytes()

		if got, want, ok := testutil.Compare(output, v.output); !ok {
			t.Errorf("test %d (%s), mismatching data:\ngot  %s\nwant %s", i, v.desc, got, want)
		}
		if len(output) != int(mw.OutputOffset) {
			t.Errorf("test %d (%s), mismatching offset: got %d, want %d", i, v.desc, len(output), mw.OutputOffset)
		}
		if v.errf != "" && !errFuncs[v.errf](err) {
			t.Errorf("test %d (%s), mismatching error:\ngot %v\nwant %s(got) == true", i, v.desc, err, v.errf)
		} else if v.errf == "" && err != nil {
			t.Errorf("test %d (%s), unexpected error: got %v", i, v.desc, err)
		}
	}
}

type faultyWriter struct{}

func (faultyWriter) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

func TestWriterReset(t *testing.T) {
	bb := new(bytes.Buffer)
	mw := NewWriter(bb)
	buf := make([]byte, 512)

	// Test Writer for idempotent Close.
	if err := mw.Close(); err != nil {
		t.Errorf("unexpected error, Close() = %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Errorf("unexpected error, Close() = %v", err)
	}
	if _, err := mw.Write(buf); err != errClosed {
		t.Errorf("unexpected error, Write(...) = %v, want %v", err, errClosed)
	}

	// Test Writer with faulty writer.
	mw.Reset(faultyWriter{})
	if _, err := mw.Write(buf); err != io.ErrShortWrite {
		t.Errorf("unexpected error, Write(...) = %v, want %v", err, io.ErrShortWrite)
	}
	if err := mw.Close(); err != io.ErrShortWrite {
		t.Errorf("unexpected error, Close() = %v, want %v", err, io.ErrShortWrite)
	}

	// Test Writer in normal use.
	bb.Reset()
	mw.Reset(bb)
	data := []byte("The quick brown fox jumped over the lazy dog.")
	cnt, err := mw.Write(data)
	if err != nil {
		t.Errorf("unexpected error, Write(...) = %v", err)
	}
	if cnt != len(data) {
		t.Errorf("write count mismatch, got %d, want %d", cnt, len(data))
	}
	if err := mw.Close(); err != nil {
		t.Errorf("unexpected error, Close() = %v", err)
	}
	if mw.InputOffset != int64(len(data)) {
		t.Errorf("input offset mismatch, got %d, want %d", mw.InputOffset, len(data))
	}
	if mw.OutputOffset != int64(bb.Len()) {
		t.Errorf("output offset mismatch, got %d, want %d", mw.OutputOffset, bb.Len())
	}
}

func BenchmarkWriter(b *testing.B) {
	data := make([]byte, 1<<16)
	rand.Read(data)

	rd := new(bytes.Reader)
	bb := new(bytes.Buffer)
	mw := new(Writer)

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rd.Reset(data)
		bb.Reset()
		mw.Reset(bb)

		cnt, err := io.Copy(mw, rd)
		if cnt != int64(len(data)) || err != nil {
			b.Fatalf("Copy() = (%d, %v), want (%d, nil)", cnt, err, len(data))
		}
		if err := mw.Close(); err != nil {
			b.Fatalf("Close() = %v, want nil", err)
		}
	}
}
