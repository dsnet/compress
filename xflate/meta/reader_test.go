// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import (
	"bytes"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

// TestReader tests that the reader is able to properly decode a set of valid
// input strings or properly detect corruption in a set of invalid input
// strings. A third-party decoder should verify that it has the same behavior
// when processing these input vectors.
func TestReader(t *testing.T) {
	var dh = testutil.MustDecodeHex

	var vectors = []struct {
		desc   string    // Description of the test
		input  []byte    // Test input string
		output []byte    // Expected output string
		final  FinalMode // Expected FinalMode value
		err    error     // Expected error
	}{{
		desc:   "empty string",
		input:  dh(""),
		output: dh(""),
		err:    io.EOF,
	}, {
		desc:   "bad empty meta block (FinalNil, first symbol not symZero)",
		input:  dh("24408705000000faffe476e0"),
		output: dh(""),
		err:    ErrCorrupt,
	}, {
		desc:   "empty meta block (FinalNil)",
		input:  dh("1c408705000000f2ffc7ede0"),
		output: dh(""),
		final:  FinalNil,
	}, {
		desc:   "empty meta block (FinalMeta)",
		input:  dh("1c408705000000d2ff1fb7e1"),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc:   "bad empty meta block, contains the magic value mid way",
		input:  dh("0580870500000080040004008605ff7f07ca"),
		output: dh(""),
		err:    ErrCorrupt,
	}, {
		desc:   "meta block containing the string 'a'",
		input:  dh("1400870500004882a0febfb4bdf0"),
		output: dh("61"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the string 'ab'",
		input:  dh("1400870500004884a008f5ff9bedf0"),
		output: dh("6162"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the string 'abc'",
		input:  dh("14c0860500202904452885faffbaf6def8"),
		output: dh("616263"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the string 'Hello, world!'",
		input:  dh("148086058024059144a1144a692894eca8541a8aa8500a5182de6f2ffc"),
		output: dh("48656c6c6f2c20776f726c6421"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the hex-string '00'*4",
		input:  dh("3440870500000012faffe026e0"),
		output: dh("00000000"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the hex-string '00'*8",
		input:  dh("2c40870500000012f4ffbf4de0"),
		output: dh("0000000000000000"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the hex-string '00'*16",
		input:  dh("2440870500000012e8ff7b9be0"),
		output: dh("00000000000000000000000000000000"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the hex-string 'ff'*4",
		input:  dh("2c40870500000052f4ffc32de0"),
		output: dh("ffffffff"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the hex-string 'ff'*8",
		input:  dh("2440870500000052e8ff835be0"),
		output: dh("ffffffffffffffff"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the hex-string 'ff'*16",
		input:  dh("1c40870500000052d0ffffb6e0"),
		output: dh("ffffffffffffffffffffffffffffffff"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the random hex-string '911fe47084a4668b'",
		input:  dh("1c808605800409d1045141852022294a09fd7f417befbd07fc"),
		output: dh("911fe47084a4668b"),
		final:  FinalMeta,
	}, {
		desc:   "meta block containing the random hex-string 'de9fa94cb16f40fc'",
		input:  dh("24808605801412641725294a2a02d156fdff447befbd0bfc"),
		output: dh("de9fa94cb16f40fc"),
		final:  FinalMeta,
	}, {
		desc:   "empty meta block with a huffLen of 1",
		input:  dh("34c087050000000020fdff7480"),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc:   "empty meta block with a huffLen of 2",
		input:  dh("3c80870500000080f47ffd1cc0"),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc:   "empty meta block with a huffLen of 3",
		input:  dh("24408705000000d2ff55f571e0"),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc:   "empty meta block with a huffLen of 4",
		input:  dh("0c008705000048ff575555d5b7f1"),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc:   "empty meta block with a huffLen of 5",
		input:  dh("34c086050020fd5f555555555555555f06f8"),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc:   "empty meta block with a huffLen of 6",
		input:  dh("1c80860580f47f5555555555555555555555555555557d15fc"),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc:   "empty meta block with a huffLen of 7",
		input:  dh("14408605d2eb5555555555555555555555555555555555555555555555555555555555555515fe"),
		output: dh(""),
		final:  FinalMeta,
	}, {
		desc:   "shortest meta block",
		input:  dh("1c408705000000f2ffc7ede0"),
		output: dh(""),
	}, {
		desc:   "longest meta block",
		input:  dh("04408605c218638c31c618638c31c618638c31c618638c31c618638c31c6185555555555555555555555555555555555555555555555555555555555555555fe"),
		output: dh(""),
	}, {
		desc:  "meta block truncated short",
		input: dh("1c8086"),
		err:   io.ErrUnexpectedEOF,
	}, {
		desc:  "meta block truncated medium-short",
		input: dh("1c808605"),
		err:   io.ErrUnexpectedEOF,
	}, {
		desc:  "meta block truncated medium-long",
		input: dh("1c808605800409d10451418520"),
		err:   io.ErrUnexpectedEOF,
	}, {
		desc:  "meta block truncated long",
		input: dh("1c808605800409d1045141852022294a09fd7f417befbd07"),
		err:   io.ErrUnexpectedEOF,
	}, {
		desc:  "random junk",
		input: dh("911fe47084a4668b"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with invalid number of HCLen codes of 6",
		input: dh("340086050000000020fdff7480"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with invalid HCLen code in the middle",
		input: dh("34c087051000000020fdff7480"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with invalid HCLen code at the end",
		input: dh("34c087050000000060fdff7480"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block first symbol being a last repeater",
		input: dh("34c0870500000000a0d1ff4f0708"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with too many symbols",
		input: dh("34c087050000000020fdff7f80"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with too few symbols",
		input: dh("34c087050000000020fe7f3a40"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with first symbol not a zero",
		input: dh("34c0870500000000a0fcff7480"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with no EOM symbol",
		input: dh("34c087050000000020fd7f740001"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with FinalStream set, but not FinalMeta",
		input: dh("35c087050000000020faffe80001"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with some padding bits not zero",
		input: dh("34c087050000000020fdff742001"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with the HDist tree not empty",
		input: dh("34c087050000000020fdff744001"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with extra symbols before EOM",
		input: dh("34c087050000000020fdff740002"),
		err:   ErrCorrupt,
	}, {
		desc:  "meta block with wrong number of padding bits",
		input: dh("2cc087050000000020fdff7440"),
		err:   ErrCorrupt,
	}}

	for i, v := range vectors {
		mr := NewReader(bytes.NewReader(v.input))
		err := mr.decodeBlock()
		output := mr.buf

		if !bytes.Equal(output, v.output) {
			t.Errorf("test %d (%s), mismatching data:\ngot  %x\nwant %x", i, v.desc, output, v.output)
		}
		if int(mr.InputOffset) != len(v.input) && err == nil {
			t.Errorf("test %d (%s), mismatching offset: got %d, want %d", i, v.desc, mr.InputOffset, len(v.input))
		}
		if mr.final != v.final {
			t.Errorf("test %d (%s), mismatching final mode: got %d, want %d", i, v.desc, mr.final, v.final)
		}
		if err != v.err {
			t.Errorf("test %d (%s), unexpected error: got %v, want %v", i, v.desc, err, v.err)
		}
	}
}

func TestReaderReset(t *testing.T) {
	mr := NewReader(nil)
	buf := make([]byte, 512)

	// Test Reader for idempotent Close.
	if err := mr.Close(); err != nil {
		t.Errorf("unexpected error: Close() = %v", err)
	}
	if err := mr.Close(); err != nil {
		t.Errorf("unexpected error: Close() = %v", err)
	}
	if _, err := mr.Read(buf); err != errClosed {
		t.Errorf("unexpected error: Read() = %v, want %v", err, errClosed)
	}

	// Test Reader with corrupt data.
	mr.Reset(strings.NewReader("corrupt"))
	if _, err := mr.Read(buf); err != ErrCorrupt {
		t.Errorf("unexpected error: Read() = %v, want %v", err, ErrCorrupt)
	}
	if err := mr.Close(); err != ErrCorrupt {
		t.Errorf("unexpected error: Close() = %v, want %v", err, ErrCorrupt)
	}

	// Test Reader on multiple back-to-back streams.
	var data = testutil.MustDecodeHex("" +
		"3c408605b22a928c944499112a4925520aa5a4cc108aa834944a45a5cc509486" +
		"321a66484a524929ab92284499d150667bef00fe2c4086059290524914519919" +
		"a98c94449919a564146988869911a5a15414959e6aefbdf7de7bef02fe3c4086" +
		"05b22a8a34145149949432235256a5a82495943233a234144a6928a232a3a844" +
		"0aa9ccc8282925514885929dd9debb00fe3d408605125a280d45a51495442914" +
		"52491452492aa23223a31025525625528a4aa1448a4aa9283312855222855454" +
		"faa0bd01fe2c408605422a421aca0c95d250486546a13494949252928a4a8994" +
		"42c928a492283384120d338aca48c928a212292aa5a2ecf602fe34408605422a" +
		"2b524aa2a49486222ad1502a3b2245a514155249948c42144a76149591925144" +
		"255166a4944449290d4554667b02fe34408605a2226534552a52465351911189" +
		"4844120a91125191069590128508452175527befbdf7de01fe",
	)
	var vectors = []struct {
		data                   string
		inOff, outOff, numBlks int64
		final                  FinalMode
	}{{
		"The quick brown fox jumped over the lazy dog.",
		93, 45, 2, FinalMeta,
	}, {
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		104, 56, 2, FinalStream,
	}, {
		"Do not communicate by sharing memory; instead, share memory by communicating.",
		148, 77, 3, FinalNil,
	}}

	rd := bytes.NewReader(data)
	for i, v := range vectors {
		mr.Reset(rd)
		buf, err := ioutil.ReadAll(mr)
		if err != nil {
			t.Errorf("test %d, unexpected error: ReadAll() = %v", i, err)
		}
		if str := string(buf); str != v.data {
			t.Errorf("test %d, output mismatch:\ngot  %s\nwant %s", i, str, v.data)
		}
		if err := mr.Close(); err != nil {
			t.Errorf("test %d, unexpected error: Close() = %v", i, err)
		}
		if mr.InputOffset != v.inOff {
			t.Errorf("test %d, input offset mismatch, got %d, want %d", i, mr.InputOffset, v.inOff)
		}
		if mr.OutputOffset != v.outOff {
			t.Errorf("test %d, output offset mismatch, got %d, want %d", i, mr.OutputOffset, v.outOff)
		}
		if mr.NumBlocks != v.numBlks {
			t.Errorf("test %d, block count mismatch, got %d, want %d", i, mr.NumBlocks, v.numBlks)
		}
		if mr.FinalMode != v.final {
			t.Errorf("test %d, final mode mismatch, got %d, want %d", i, mr.FinalMode, v.final)
		}
	}
}

func BenchmarkReader(b *testing.B) {
	data := make([]byte, 1<<16)
	rand.Read(data)
	bb := bytes.NewBuffer(nil)
	mr := NewReader(nil)

	mw := NewWriter(bb)
	mw.Write(data)
	mw.Close()

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rd := bytes.NewReader(bb.Bytes())
		mr.Reset(rd)

		cnt, err := io.Copy(ioutil.Discard, mr)
		if cnt != int64(len(data)) {
			b.Fatalf("mismatching count: Copy() = %d, want %d", cnt, len(data))
		}
		if err != nil {
			b.Fatalf("unexpected error: Copy() = %v", err)
		}
		if err := mr.Close(); err != nil {
			b.Fatalf("unexpected error: Close() = %v", err)
		}
	}
}
