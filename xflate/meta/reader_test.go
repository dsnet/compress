// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import "io"
import "runtime"
import "encoding/hex"
import "github.com/dsnet/golib/bits"
import "github.com/stretchr/testify/assert"
import "testing"

// TestReader tests that the reader is able to properly decode a set of valid
// input strings or properly detect corruption in a set of invalid input
// strings. A third-party decoder should verify that it has the same behavior
// when processing these input vectors.
func TestReader(t *testing.T) {
	var vectors = []struct {
		desc   string   // Description of the test
		input  string   // Test input string in hex
		output string   // Expected output string in hex
		last   LastMode // Expected LastMode value
		pos    int      // Expected reverse search position
		err    error    // Expected error
	}{{
		"empty string",
		"", "",
		LastNil, -1, io.EOF,
	}, {
		"empty meta block",
		"1c408705000000d2ff1fb7e1", "",
		LastMeta, 0, nil,
	}, {
		"empty meta block, contains the magic value mid way",
		"0580870500000080040004008605ff7f07ca", "",
		LastStream, 10, nil,
	}, {
		"meta block containing the string 'a'",
		"1400870500004882a0febfb4bdf0", "61",
		LastMeta, 0, nil,
	}, {
		"meta block containing the string 'ab'",
		"1400870500004884a008f5ff9bedf0", "6162",
		LastMeta, 0, nil,
	}, {
		"meta block containing the string 'abc'",
		"14c0860500202904452885faffbaf6def8", "616263",
		LastMeta, 0, nil,
	}, {
		"meta block containing the string 'Hello, world!'",
		"148086058024059144a1144a692894eca8541a8aa8500a5182de6f2ffc", "48656c6c6f2c20776f726c6421",
		LastMeta, 0, nil,
	}, {
		"meta block containing the hex-string '00'*4",
		"3440870500000012faffe026e0", "00000000",
		LastMeta, 0, nil,
	}, {
		"meta block containing the hex-string '00'*8",
		"2c40870500000012f4ffbf4de0", "0000000000000000",
		LastMeta, 0, nil,
	}, {
		"meta block containing the hex-string '00'*16",
		"2440870500000012e8ff7b9be0", "00000000000000000000000000000000",
		LastMeta, 0, nil,
	}, {
		"meta block containing the hex-string 'ff'*4",
		"2c40870500000052f4ffc32de0", "ffffffff",
		LastMeta, 0, nil,
	}, {
		"meta block containing the hex-string 'ff'*8",
		"2440870500000052e8ff835be0", "ffffffffffffffff",
		LastMeta, 0, nil,
	}, {
		"meta block containing the hex-string 'ff'*16",
		"1c40870500000052d0ffffb6e0", "ffffffffffffffffffffffffffffffff",
		LastMeta, 0, nil,
	}, {
		"meta block containing the random hex-string '911fe47084a4668b'",
		"1c808605800409d1045141852022294a09fd7f417befbd07fc", "911fe47084a4668b",
		LastMeta, 0, nil,
	}, {
		"meta block containing the random hex-string 'de9fa94cb16f40fc'",
		"24808605801412641725294a2a02d156fdff447befbd0bfc", "de9fa94cb16f40fc",
		LastMeta, 0, nil,
	}, {
		"empty meta block with a huffLen of 1",
		"34c087050000000020fdff7480", "",
		LastMeta, 0, nil,
	}, {
		"empty meta block with a huffLen of 2",
		"3c80870500000080f47ffd1cc0", "",
		LastMeta, 0, nil,
	}, {
		"empty meta block with a huffLen of 3",
		"24408705000000d2ff55f571e0", "",
		LastMeta, 0, nil,
	}, {
		"empty meta block with a huffLen of 4",
		"0c008705000048ff575555d5b7f1", "",
		LastMeta, 0, nil,
	}, {
		"empty meta block with a huffLen of 5",
		"34c086050020fd5f555555555555555f06f8", "",
		LastMeta, 0, nil,
	}, {
		"empty meta block with a huffLen of 6",
		"1c80860580f47f5555555555555555555555555555557d15fc", "",
		LastMeta, 0, nil,
	}, {
		"empty meta block with a huffLen of 7",
		"14408605d2eb5555555555555555555555555555555555555555555555555555555555555515fe", "",
		LastMeta, 0, nil,
	}, {
		"shortest meta block",
		"1c408705000000f2ffc7ede0", "",
		LastNil, 0, nil,
	}, {
		"longest meta block",
		"04408605c218638c31c618638c31c618638c31c618638c31c618638c31c6185555555555555555555555555555555555555555555555555555555555555555fe", "",
		LastNil, 0, nil,
	}, {
		"meta block truncated short",
		"1c8086", "",
		LastMeta, 0, io.ErrUnexpectedEOF,
	}, {
		"meta block truncated medium-short",
		"1c808605", "",
		LastMeta, 0, io.ErrUnexpectedEOF,
	}, {
		"meta block truncated medium-long",
		"1c808605800409d10451418520", "",
		LastMeta, 0, io.ErrUnexpectedEOF,
	}, {
		"meta block truncated long",
		"1c808605800409d1045141852022294a09fd7f417befbd07", "",
		LastMeta, 0, io.ErrUnexpectedEOF,
	}, {
		"random junk",
		"911fe47084a4668b", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with invalid number of HCLen codes of 6",
		"340086050000000020fdff7480", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with invalid HCLen code in the middle",
		"34c087051000000020fdff7480", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with invalid HCLen code at the end",
		"34c087050000000060fdff7480", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block first symbol being a last repeater",
		"34c0870500000000a0d1ff4f0708", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with too many symbols",
		"34c087050000000020fdff7f80", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with too few symbols",
		"34c087050000000020fe7f3a40", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with first symbol not a zero",
		"34c0870500000000a0fcff7480", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with no EOM symbol",
		"34c087050000000020fd7f740001", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with LastStream set, but not LastMeta",
		"35c087050000000020faffe80001", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with some padding bits not zero",
		"34c087050000000020fdff742001", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with the HDist tree not empty",
		"34c087050000000020fdff744001", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with extra symbols before EOM",
		"34c087050000000020fdff740002", "",
		LastNil, -1, errMetaCorrupt,
	}, {
		"meta block with wrong number of padding bits",
		"2cc087050000000020fdff7440", "",
		LastNil, -1, errMetaCorrupt,
	}}

	var mr Reader
	for i, v := range vectors {
		input, _ := hex.DecodeString(v.input)
		data, last, cnt, err := mr.decodeBlock(bits.NewBuffer(input))
		output := hex.EncodeToString(data)
		pos := ReverseSearch(input)

		var fmt = "Check '%s' in trial %d: %s"
		if err == nil {
			assert.Equal(t, len(v.input)/2, cnt, fmt, "cnt", i, v.desc)
			assert.Equal(t, v.output, output, fmt, "output", i, v.desc)
			assert.Equal(t, v.last, last, fmt, "last", i, v.desc)
			assert.Equal(t, v.pos, pos, fmt, "pos", i, v.desc)
		}
		assert.Equal(t, v.err, err, fmt, "err", i, v.desc)
	}
}

func BenchmarkReader(b *testing.B) {
	data := randBytes(1 << 16) // 64kiB
	data2 := make([]byte, len(data))
	buf := bits.NewBuffer(nil)

	mw := NewWriter(buf, LastStream)
	mw.Write(data)
	mw.Close()

	bb := bits.NewBuffer(nil)
	mr := NewReader(nil)

	runtime.GC()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for idx := 0; idx < b.N; idx++ {
		bb.ResetBuffer(buf.Bytes())
		mr.Reset(bb)
		mr.Read(data2)
	}
}
