// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"io"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"
)

func TestReader(t *testing.T) {
	// To verify any of these hexstrings as valid or invalid DEFLATE streams
	// according to the C zlib library, you can use the Python wrapper library:
	//	>>> hex_string = "010100feff11"
	//	>>> import zlib
	//	>>> zlib.decompress(hex_string.decode("hex"), -15) # Negative means raw DEFLATE
	//	'\x11'

	var vectors = []struct {
		desc   string // Description of the test
		input  string // Test input string in hex
		output string // Expected output string in hex
		inIdx  int64  // Expected input offset after reading
		outIdx int64  // Expected output offset after reading
		err    error  // Expected error
	}{{
		desc: "empty string (truncated)",
		err:  io.ErrUnexpectedEOF,
	}, {
		desc: "degenerate HCLenTree",
		input: "05e0010000000000100000000000000000000000000000000000000000000000" +
			"00000000000000000004",
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, empty HLitTree, empty HDistTree",
		input: "05e0010400000000000000000000000000000000000000000000000000000000" +
			"00000000000000000010",
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "empty HCLenTree",
		input: "05e0010000000000000000000000000000000000000000000000000000000000" +
			"00000000000000000010",
		inIdx: 10,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree, use missing HDist symbol",
		input: "000100feff000de0010400000000100000000000000000000000000000000000" +
			"0000000000000000000000000000002c",
		output: "00",
		inIdx:  48,
		outIdx: 1,
		err:    ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, degenerate HDistTree, use missing HDist symbol",
		input: "000100feff000de0010000000000000000000000000000000000000000000000" +
			"00000000000000000610000000004070",
		output: "00",
		inIdx:  16,
		outIdx: 1,
		err:    ErrCorrupt,
	}, {
		desc: "complete HCLenTree, empty HLitTree, empty HDistTree",
		input: "05e0010400000000100400000000000000000000000000000000000000000000" +
			"0000000000000000000000000008",
		output: "00000000000000000000000000000000000000000000000000000000000000",
		inIdx:  46,
		outIdx: 31,
		err:    ErrCorrupt,
	}, {
		desc: "complete HCLenTree, empty HLitTree, degenerate HDistTree",
		input: "05e0010400000000100400000000000000000000000000000000000000000000" +
			"0000000000000000000800000008",
		output: "00000000000000000000000000000000000000000000000000000000000000",
		inIdx:  46,
		outIdx: 31,
		err:    ErrCorrupt,
	}, {
		desc: "complete HCLenTree, degenerate HLitTree, degenerate HDistTree, use missing HLit symbol",
		input: "05e0010400000000100000000000000000000000000000000000000000000000" +
			"0000000000000000001c",
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, too large HDistTree",
		input: "edff870500000000200400000000000000000000000000000000000000000000" +
			"000000000000000000080000000000000004",
		inIdx: 3,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree, excessive repeater code",
		input: "edfd870500000000200400000000000000000000000000000000000000000000" +
			"000000000000000000e8b100",
		inIdx: 43,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree of normal length 30",
		input: "05fd01240000000000f8ffffffffffffffffffffffffffffffffffffffffffff" +
			"ffffffffffffffffff07000000fe01",
		output: "",
		inIdx:  47,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree of excessive length 31",
		input: "05fe01240000000000f8ffffffffffffffffffffffffffffffffffffffffffff" +
			"ffffffffffffffffff07000000fc03",
		inIdx: 3,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, over-subscribed HLitTree, empty HDistTree",
		input: "05e001240000000000fcffffffffffffffffffffffffffffffffffffffffffff" +
			"ffffffffffffffffff07f00f",
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, under-subscribed HLitTree, empty HDistTree",
		input: "05e001240000000000fcffffffffffffffffffffffffffffffffffffffffffff" +
			"fffffffffcffffffff07f00f",
		inIdx: 42,
		err:   ErrCorrupt,
	}, {
		desc: "complete HCLenTree, complete HLitTree with single code, empty HDistTree",
		input: "05e001240000000000f8ffffffffffffffffffffffffffffffffffffffffffff" +
			"ffffffffffffffffff07f00f",
		output: "01",
		inIdx:  44,
		outIdx: 1,
	}, {
		desc: "complete HCLenTree, complete HLitTree with multiple codes, empty HDistTree",
		input: "05e301240000000000f8ffffffffffffffffffffffffffffffffffffffffffff" +
			"ffffffffffffffffff07807f",
		output: "01",
		inIdx:  44,
		outIdx: 1,
	}, {
		desc: "complete HCLenTree, complete HLitTree, degenerate HDistTree, use valid HDist symbol",
		input: "000100feff000de0010400000000100000000000000000000000000000000000" +
			"0000000000000000000000000000003c",
		output: "00000000",
		inIdx:  48,
		outIdx: 4,
	}, {
		desc: "complete HCLenTree, degenerate HLitTree, degenerate HDistTree",
		input: "05e0010400000000100000000000000000000000000000000000000000000000" +
			"0000000000000000000c",
		inIdx: 42,
	}, {
		desc: "complete HCLenTree, degenerate HLitTree, empty HDistTree",
		input: "05e0010400000000100000000000000000000000000000000000000000000000" +
			"00000000000000000004",
		inIdx: 42,
	}, {
		desc: "complete HCLenTree, complete HLitTree, empty HDistTree, spanning repeater code",
		input: "edfd870500000000200400000000000000000000000000000000000000000000" +
			"000000000000000000e8b000",
		inIdx: 43,
	}, {
		desc: "complete HCLenTree with length codes, complete HLitTree, empty HDistTree",
		input: "ede0010400000000100000000000000000000000000000000000000000000000" +
			"0000000000000000000400004000",
		inIdx: 46,
	}, {
		desc: "complete HCLenTree, complete HLitTree, degenerate HDistTree, use valid HLit symbol 284 with count 31",
		input: "000100feff00ede0010400000000100000000000000000000000000000000000" +
			"000000000000000000000000000000040000407f00",
		output: "0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000" +
			"000000",
		inIdx:  53,
		outIdx: 259,
	}, {
		desc:   "complete HCLenTree, complete HLitTree, degenerate HDistTree, use valid HLit and HDist symbols",
		input:  "0cc2010d00000082b0ac4aff0eb07d27060000ffff",
		output: "616263616263",
		inIdx:  21,
		outIdx: 6,
	}, {
		desc:   "fixed block, use reserved symbol 287",
		input:  "33180700",
		output: "30",
		inIdx:  3,
		outIdx: 1,
		err:    ErrCorrupt,
	}, {
		desc:   "raw block",
		input:  "010100feff11",
		output: "11",
		inIdx:  6,
		outIdx: 1,
	}, {
		desc:  "issue 10426 - over-subscribed HCLenTree caused a hang",
		input: "344c4a4e494d4b070000ff2e2eff2e2e2e2e2eff",
		inIdx: 5,
		err:   ErrCorrupt,
	}, {
		desc:   "issue 11030 - empty HDistTree unexpectedly leads to error",
		input:  "05c0070600000080400fff37a0ca",
		output: "",
		inIdx:  14,
	}, {
		desc: "issue 11033 - empty HDistTree unexpectedly led to error",
		input: "050fb109c020cca5d017dcbca044881ee1034ec149c8980bbc413c2ab35be9dc" +
			"b1473449922449922411202306ee97b0383a521b4ffdcf3217f9f7d3adb701",
		output: "3130303634342068652e706870005d05355f7ed957ff084a90925d19e3ebc6d0" +
			"c6d7",
		inIdx:  63,
		outIdx: 34,
	}}

	for i, v := range vectors {
		input, _ := hex.DecodeString(v.input)
		rd := NewReader(bytes.NewReader(input))
		data, err := ioutil.ReadAll(rd)
		output := hex.EncodeToString(data)

		if err != v.err {
			t.Errorf("test %d, %s\nerror mismatch: got %v, want %v", i, v.desc, err, v.err)
		}
		if output != v.output {
			t.Errorf("test %d, %s\noutput mismatch:\ngot  %v\nwant %v", i, v.desc, output, v.output)
		}
		if rd.InputOffset != v.inIdx {
			t.Errorf("test %d, %s\ninput offset mismatch: got %d, want %d", i, v.desc, rd.InputOffset, v.inIdx)
		}
		if rd.OutputOffset != v.outIdx {
			t.Errorf("test %d, %s\noutput offset mismatch: got %d, want %d", i, v.desc, rd.OutputOffset, v.outIdx)
		}
	}
}

func TestTruncatedStreams(t *testing.T) {
	const data = "\x00\f\x00\xf3\xffhello, world\x01\x00\x00\xff\xff"

	for i := 0; i < len(data)-1; i++ {
		r := NewReader(strings.NewReader(data[:i]))
		_, err := io.Copy(ioutil.Discard, r)
		if err != io.ErrUnexpectedEOF {
			t.Errorf("io.Copy(%d) on truncated stream: got %v, want %v", i, err, io.ErrUnexpectedEOF)
		}
	}
}

func benchmarkDecode(b *testing.B, testfile string) {
	b.StopTimer()
	b.ReportAllocs()

	input, err := ioutil.ReadFile("testdata/" + testfile)
	if err != nil {
		b.Fatal(err)
	}
	output, err := ioutil.ReadAll(NewReader(bytes.NewReader(input)))
	if err != nil {
		b.Fatal(err)
	}

	nb := int64(len(output))
	output = nil
	runtime.GC()

	b.SetBytes(nb)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		cnt, err := io.Copy(ioutil.Discard, NewReader(bufio.NewReader(bytes.NewReader(input))))
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
