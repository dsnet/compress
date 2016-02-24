// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import "runtime"
import "encoding/hex"
import "github.com/dsnet/golib/bits"
import "github.com/stretchr/testify/assert"
import "testing"

// TestWriter tests that the encoded output matches the expected output exactly.
// A failure here does not necessarily mean that the encoder is wrong since
// there are many possible representations. Before changing the test vectors to
// make a test pass, one must verify the new output is correct.
func TestWriter(t *testing.T) {
	var vectors = []struct {
		desc   string   // Description of the text
		input  string   // Test input string in hex
		output string   // Expected output string in hex
		last   LastMode // Input last mode
		err    error    // Expected error
	}{{
		"empty meta block",
		"", "1c408705000000d2ff1fb7e1",
		LastMeta, nil,
	}, {
		"the input string 'a'",
		"61", "1400870500004882a0febfb4bdf0",
		LastMeta, nil,
	}, {
		"the input string 'ab'",
		"6162", "1400870500004884a008f5ff9bedf0",
		LastMeta, nil,
	}, {
		"the input string 'abc'",
		"616263", "14c0860500202904452885faffbaf6def8",
		LastMeta, nil,
	}, {
		"the input string 'Hello, world!' with LastNil",
		"48656c6c6f2c20776f726c6421", "1c80860580908248a2500aa534144a76542a0d455428852841e7b727fc",
		LastNil, nil,
	}, {
		"the input string 'Hello, world!' with LastMeta",
		"48656c6c6f2c20776f726c6421", "148086058024059144a1144a692894eca8541a8aa8500a5182de6f2ffc",
		LastMeta, nil,
	}, {
		"the input string 'Hello, world!' with LastStream",
		"48656c6c6f2c20776f726c6421", "158086058024059144a1144a692894eca8541a8aa8500a5182de6f2ffc",
		LastStream, nil,
	}, {
		"the input hex-string '00'*4",
		"00000000", "3440870500000012faffe026e0",
		LastMeta, nil,
	}, {
		"the input hex-string '00'*8",
		"0000000000000000", "2c40870500000012f4ffbf4de0",
		LastMeta, nil,
	}, {
		"the input hex-string '00'*16",
		"00000000000000000000000000000000", "2440870500000012e8ff7b9be0",
		LastMeta, nil,
	}, {
		"the input hex-string 'ff'*4",
		"ffffffff", "2c40870500000052f4ffc32de0",
		LastMeta, nil,
	}, {
		"the input hex-string 'ff'*8",
		"ffffffffffffffff", "2440870500000052e8ff835be0",
		LastMeta, nil,
	}, {
		"the input hex-string 'ff'*16",
		"ffffffffffffffffffffffffffffffff", "1c40870500000052d0ffffb6e0",
		LastMeta, nil,
	}, {
		"the random hex-string '911fe47084a4668b'",
		"911fe47084a4668b", "1c808605800409d1045141852022294a09fd7f417befbd07fc",
		LastMeta, nil,
	}, {
		"the random hex-string 'de9fa94cb16f40fc'",
		"de9fa94cb16f40fc", "24808605801412641725294a2a02d156fdff447befbd0bfc",
		LastMeta, nil,
	}, {
		"the input hex-string '55'*22",
		"55555555555555555555555555555555555555555555", "0540860512a52449922449922449922449922449922449922449922449922449922449922449d237edbdf79efe",
		LastStream, nil,
	}, {
		"the input hex-string '55'*23",
		"5555555555555555555555555555555555555555555555", "04408605924a499224499224499224499224499224499224499224499224499224499224499224493aa6bdf7defe",
		LastMeta, nil,
	}, {
		"the input hex-string '55'*24",
		"555555555555555555555555555555555555555555555555", "3c408605322b4992244992244992244992244992244992244992244992244992244992244992244992f4487bef3d01fe",
		LastNil, nil,
	}, {
		"the input hex-string '55'*25",
		"55555555555555555555555555555555555555555555555555", "3540860592a824499224499224499224499224499224499224499224499224499224499224499224499224fdd1de7b02fe",
		LastStream, nil,
	}, {
		"the input hex-string '55'*26",
		"5555555555555555555555555555555555555555555555555555", "2c40860512a92449922449922449922449922449922449922449922449922449922449922449922449922449d217edbd03fe",
		LastMeta, nil,
	}, {
		"the input hex-string '55'*27",
		"555555555555555555555555555555555555555555555555555555", "1c40860542a924499224499224499224499224499224499224499224499224499224499224499224499224499224fdd0de03fe",
		LastNil, nil,
	}, {
		"the input hex-string '55'*28",
		"55555555555555555555555555555555555555555555555555555555", "2d408605121a9224499224499224499224499224499224499224499224499224499224499224499224499224499224e983f604fe",
		LastStream, nil,
	}, {
		"the input hex-string '55'*29",
		"5555555555555555555555555555555555555555555555555555555555", "2c4086059234244992244992244992244992244992244992244992244992244992244992244992244992244992244992241dd006fe",
		LastMeta, nil,
	}, {
		"the input hex-string '55'*30",
		"555555555555555555555555555555555555555555555555555555555555", "0440860582962449922449922449922449922449922449922449922449922449922449922449922449922449922449922449321bfe",
		LastNil, nil,
	}, {
		"the input hex-string '55'*31",
		"55555555555555555555555555555555555555555555555555555555555555", "",
		LastStream, errMetaInvalid,
	}, {
		"the input hex-string '55'*32",
		"5555555555555555555555555555555555555555555555555555555555555555", "",
		LastMeta, errMetaInvalid,
	}, {
		"the input hex-string '73de76bebcf69d5fed3fb3cee87bacfd7de876facffedf'",
		"73de76bebcf69d5fed3fb3cee87bacfd7de876facffedf", "2480860580688842414428908244209499443645915054024145223bd01022946c8a1ee8b50afc",
		LastNil, nil,
	}, {
		"the input hex-string '73de76bebcf69d5fed3fb3cee87bacfd7de876facffede'",
		"73de76bebcf69d5fed3fb3cee87bacfd7de876facffede", "",
		LastStream, errMetaInvalid,
	}, {
		"the input hex-string 'def773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9'",
		"def773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9", "2480860580941604320b992189a4a10492fe088164662b082102158448a06ce98508b2eb862245b60afc",
		LastMeta, nil,
	}, {
		"the input hex-string 'dff773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9'",
		"dff773bfab15d257ffffffbbafdf3fef6e1fefd6e75ffffff6fefcff67d9", "",
		LastMeta, errMetaInvalid,
	}}

	var mw Writer
	for i, v := range vectors {
		var b bits.Buffer
		input, _ := hex.DecodeString(v.input)
		cnt, err := mw.encodeBlock(&b, input, v.last)
		output := hex.EncodeToString(b.Bytes())

		fmt := "Check '%s' in trial %d: %s"
		if err == nil {
			assert.Equal(t, v.output, output, fmt, "output", i, v.desc)
		}
		assert.Equal(t, len(b.Bytes()), cnt, fmt, "cnt", i, v.desc)
		assert.Equal(t, v.err, err, fmt, "err", i, v.desc)
	}
}

func BenchmarkWriter(b *testing.B) {
	data := randBytes(1 << 16) // 64kiB
	bb := bits.NewBuffer(nil)
	mw := NewWriter(nil, LastStream)

	runtime.GC()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for idx := 0; idx < b.N; idx++ {
		bb.Reset()
		mw.Reset(bb, LastStream)
		mw.Write(data)
		mw.Close()
	}
}
