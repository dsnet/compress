// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"io"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/dsnet/compress"
	"github.com/dsnet/compress/internal"
)

type rand struct {
	cipher.Block
	blk [aes.BlockSize]byte
}

func newRand() *rand {
	r, _ := aes.NewCipher(make([]byte, aes.BlockSize))
	return &rand{Block: r}
}

func (r *rand) Int() (x int) {
	r.Encrypt(r.blk[:], r.blk[:])
	x |= int(r.blk[0]) << 0
	x |= int(r.blk[1]) << 8
	x |= int(r.blk[2]) << 16
	x |= int(r.blk[3]) << 24
	x |= int(r.blk[4]) << 32
	x |= int(r.blk[5]) << 40
	x |= int(r.blk[6]) << 48
	x |= int(r.blk[7]&0x3f) << 56
	return x
}

func (r *rand) Intn(n int) int {
	return r.Int() % n
}

func (r *rand) Bytes(n int) []byte {
	b := make([]byte, n)
	bb := b
	for len(bb) > 0 {
		r.Encrypt(r.blk[:], r.blk[:])
		cnt := copy(bb, r.blk[:])
		bb = bb[cnt:]
	}
	return b
}

func (r *rand) Perm(n int) []int {
	m := make([]int, n)
	for i := 0; i < n; i++ {
		j := r.Intn(i + 1)
		m[i] = m[j]
		m[j] = i
	}
	return m
}

func hexDecode(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func reverseBytes(b []byte) []byte {
	for i, c := range b {
		b[i] = internal.ReverseLUT[c]
	}
	return b
}

const testSize = 1000

var (
	testVector = hexDecode("" +
		"f795bd4a52e2ffae29ddff1335428e2b267fb5ff9e6339fdebbfa9c4aa3aff1e" +
		"d5af474b5212fc9b81eaff6e1cfc0bea797b45cc128eabe9ff6ecbb2fe25076a" +
		"b6e0950b2bb3d6d35dff9ec1744f3e7ff816d299c3141190bafe4cb1ff1e2fdc" +
		"ec275b791165baed4cf7be85151dffeeead3d637db3d0e14fefd6ca32381b344" +
		"df53dfff2ef3006fe37486ff8cf37ceeff2e07ff5ee3db8f9e0860ffce4476fe" +
		"41eca8f4d62b6894c64ecd7c8d290a1dff1e3afc9f01d791ff9eadff212106dd" +
		"4d622bebd70dd07dff9ebbbe003f60ff9ef23f055b2db219ae1a1f9d75661178" +
		"6510f9ff1e2cccbf1f0d3c95ff6ef47f0c3614fffb166d0f6144843b16ffeef1" +
		"7f0768998ba7a634c5ff1ebcfaff03f059ffcef173fb093ac94b3560627d3455" +
		"9450ff9e9bff5701fd6c3b88626ed0fbd06a70dcff1ef8f8bf06e63b7af9f17b" +
		"cdd81bdf76ff9ea6f7ff6700f09a492bfdd44f022097b252ff9ef3fa1f03b7a7" +
		"62796af15d8979034a60d163ff6edb7f10afe5399bffeef7bf0da1c8921b9f39" +
		"58ffaefcf97f0733027902b4afecc849c4eacc2bb97fffaef0ff25ac17dbd337" +
		"9bff2ef8fc570005515d167773ee3d55ff6e12ff8701a70a31ff1e2b8b3e271c" +
		"ede640faffeed57f1cd5f93da995ff1ebaf01d590e010a5effaec4fe7bff1ed6" +
		"67ff63ff9e7eeb7f14b20724a6194d553fff0e1851530a20ff2e3b741d019cbb" +
		"3197ff870c53de92922192ff6e956bf40f0390ff2e61dbff2ad7ea156242c7a6" +
		"c3fe3157e970f4ffaefd5f00020fe74db5bd2affeee785628ed157159c0727d3" +
		"c4ff1e47c72b1f407859c9e3443076efdb10ab47ff9e71a6fd4f0148bd816802" +
		"8e8918d930ff2ee89efe4f03af44ddbb98a4918ef6ffeef8f38f023fa7bc4947" +
		"e62cdd712321c843ff1ef05d0136724cce8a4f2c80009340a2ffee15d7d6ffb3" +
		"6d5321374698e9a447b637eb5d0eff4e5afcf7bf04b923f636333b416e8586e4" +
		"81cbabff1e6f30a4ff0d7fbea24353332e05ffaeffcb017e3156ff9edd7f04a8" +
		"b328b68e67e7a5617a6ba356740aff1e6f86ff2cf751de8325ddc3ff1eda70fe" +
		"9f03d103a20e56b0403be761d47e3bff6e1ce37f0261e79070b30e1164d46904" +
		"bc00f8ff9e6ef404ba053a1e5925a5b1009e08e83c47ffceb88fff1eafda5091" +
		"e98bff6ef33077077bccf21d30ff9e6ccef9af00c6ff6e85494dff4981800726" +
		"d1cbff2efdb700f012a6ff1e53141f017bff66fcfb051861ff1e707dc6ff3996" +
		"0c2ca67d3bcbfbba33dbf3ecff2ebce0f19f032e3bd17ac4df03b2f021ecff9e" +
		"7f59da4fe9efce85a6296b6c1d28cc7d1b11ffee319aff16ec763c189247cdb0" +
		"842e860fff9eed23ff2d35ec5819070038c50ea231f32e55ffaeef5f06037e14" +
		"36b865e85fff9e2cd01e",
	)

	testCodes = func() (codes PrefixCodes) {
		for i := 0; i < 100; i++ {
			codes = append(codes, PrefixCode{Sym: uint32(len(codes)), Cnt: 0})
		}
		for i := 0; i < 25; i++ {
			codes = append(codes, PrefixCode{Sym: uint32(len(codes)), Cnt: 10})
		}
		for i := 0; i < 5; i++ {
			codes = append(codes, PrefixCode{Sym: uint32(len(codes)), Cnt: 1000})
		}
		codes.SortByCount()
		if err := GenerateLengths(codes, 15); err != nil {
			panic(err)
		}
		codes.SortBySymbol()
		if err := GeneratePrefixes(codes); err != nil {
			panic(err)
		}
		return codes
	}()

	testRanges = MakeRangeCodes(0, []uint{0, 1, 2, 3, 4})
)

func TestReader(t *testing.T) {
	var readers = map[string]func([]byte) io.Reader{
		"io.Reader": func(b []byte) io.Reader {
			return struct{ io.Reader }{bytes.NewReader(b)}
		},
		"bytes.Buffer": func(b []byte) io.Reader {
			return bytes.NewBuffer(b)
		},
		"bytes.Reader": func(b []byte) io.Reader {
			return bytes.NewReader(b)
		},
		"string.Reader": func(b []byte) io.Reader {
			return strings.NewReader(string(b))
		},
		"compress.ByteReader": func(b []byte) io.Reader {
			return struct{ compress.ByteReader }{bytes.NewReader(b)}
		},
		"compress.BufferedReader": func(b []byte) io.Reader {
			return struct{ compress.BufferedReader }{bufio.NewReader(bytes.NewReader(b))}
		},
	}
	var endians = map[string]bool{"littleEndian": false, "bigEndian": true}

	var i int
	for ne, endian := range endians {
		for nr, newReader := range readers {
			var br Reader
			buf := make([]byte, len(testVector))
			copy(buf, testVector)
			if endian {
				buf = reverseBytes(buf)
			}
			rd := newReader(buf)
			br.Init(rd, endian)

			var pd Decoder
			pd.Init(testCodes)

			r := newRand()
		loop:
			for j := 0; ; j++ {
				// Stop if we read enough bits.
				offset := 8*br.offset - int64(br.numBits)
				if br.bufRd != nil {
					discardBits := br.discardBits + int(br.fedBits-br.numBits)
					offset = 8*br.offset + int64(discardBits)
				}
				if offset > 8*testSize {
					break
				}

				switch j % 4 {
				case 0:
					// Test unaligned Read.
					if br.numBits%8 != 0 {
						cnt, err := br.Read([]byte{0})
						if cnt != 0 {
							t.Errorf("test %d, %s %s, write count mismatch: got %d, want 0", i, ne, nr, cnt)
							break loop
						}
						if err == nil {
							t.Errorf("test %d, %s %s, unexpected write success", i, ne, nr)
							break loop
						}
					}

					pads := br.ReadPads()
					if pads != 0 {
						t.Errorf("test %d, %s %s, bit padding mismatch: got %d, want 0", i, ne, nr, pads)
						break loop
					}
					want := r.Bytes(r.Intn(16))
					if endian {
						want = reverseBytes(want)
					}
					got := make([]byte, len(want))
					cnt, err := io.ReadFull(&br, got)
					if cnt != len(want) {
						t.Errorf("test %d, %s %s, read count mismatch: got %d, want %d", i, ne, nr, cnt, len(want))
						break loop
					}
					if err != nil {
						t.Errorf("test %d, %s %s, unexpected read error: got %v", i, ne, nr, err)
						break loop
					}
					if bytes.Compare(want, got) != 0 {
						t.Errorf("test %d, %s %s, read bytes mismatch:\ngot  %x\nwant %x", i, ne, nr, got, want)
						break loop
					}
				case 1:
					n := int(testRanges.End() - testRanges.Base())
					want := uint(testRanges.Base() + uint32(r.Intn(n)))
					got := br.ReadOffset(&pd, testRanges)
					if got != want {
						t.Errorf("test %d, %s %s, read offset mismatch: got %d, want %d", i, ne, nr, got, want)
						break loop
					}
				case 2:
					nb := uint(r.Intn(24))
					want := uint(r.Int() & (1<<nb - 1))
					got, ok := br.TryReadBits(nb)
					if !ok {
						got = br.ReadBits(nb)
					}
					if got != want {
						t.Errorf("test %d, %s %s, read bits mismatch: got %d, want %d", i, ne, nr, got, want)
						break loop
					}
				case 3:
					want := uint(testCodes[r.Intn(len(testCodes))].Sym)
					got, ok := br.TryReadSymbol(&pd)
					if !ok {
						got = br.ReadSymbol(&pd)
					}
					if got != want {
						t.Errorf("test %d, %s %s, read symbol mismatch: got %d, want %d", i, ne, nr, got, want)
						break loop
					}
				}
			}

			pads := br.ReadPads()
			if pads != 0 {
				t.Errorf("test %d, %s %s, bit padding mismatch: got %d, want 0", i, ne, nr, pads)
			}
			ofs, err := br.Flush()
			if br.numBits != 0 {
				t.Errorf("test %d, %s, bit buffer not drained: got %d, want < 8", i, ne, br.numBits)
			}
			if ofs != int64(len(testVector)) {
				t.Errorf("test %d, %s, offset mismatch: got %d, want %d", i, ne, ofs, len(testVector))
			}
			if err != nil {
				t.Errorf("test %d, %s, unexpected flush error: got %v", i, ne, err)
			}
			i++
		}
	}
}

func TestWriter(t *testing.T) {
	var endians = map[string]bool{"littleEndian": false, "bigEndian": true}
	endians = map[string]bool{"littleEndian": false}

	var i int
	for ne, endian := range endians {
		var bw Writer
		wr := bytes.NewBuffer(nil)
		bw.Init(wr, endian)

		var pe Encoder
		pe.Init(testCodes)

		var re RangeEncoder
		re.Init(testRanges)

		r := newRand()
	loop:
		for j := 0; 8*bw.offset+int64(8*bw.cntBuf)+int64(bw.numBits) < 8*testSize; j++ {
			switch j % 4 {
			case 0:
				// Test unaligned Write.
				if bw.numBits%8 != 0 {
					cnt, err := bw.Write([]byte{0})
					if cnt != 0 {
						t.Errorf("test %d, %s, write count mismatch: got %d, want 0", i, ne, cnt)
						break loop
					}
					if err == nil {
						t.Errorf("test %d, %s, unexpected write success", i, ne)
						break loop
					}
				}

				bw.WritePads(0)
				b := r.Bytes(r.Intn(16))
				if endian {
					b = reverseBytes(b)
				}
				cnt, err := bw.Write(b)
				if cnt != len(b) {
					t.Errorf("test %d, %s, write count mismatch: got %d, want %d", i, ne, cnt, len(b))
					break loop
				}
				if err != nil {
					t.Errorf("test %d, %s, unexpected write error: got %v", i, ne, err)
					break loop
				}
			case 1:
				n := int(testRanges.End() - testRanges.Base())
				ofs := uint(testRanges.Base() + uint32(r.Intn(n)))
				bw.WriteOffset(ofs, &pe, &re)
			case 2:
				nb := uint(r.Intn(24))
				val := uint(r.Int() & (1<<nb - 1))
				ok := bw.TryWriteBits(val, nb)
				if !ok {
					bw.WriteBits(val, nb)
				}
			case 3:
				sym := uint(testCodes[r.Intn(len(testCodes))].Sym)
				ok := bw.TryWriteSymbol(sym, &pe)
				if !ok {
					bw.WriteSymbol(sym, &pe)
				}
			}
		}

		// Flush the Writer.
		bw.WritePads(0)
		ofs, err := bw.Flush()
		if bw.numBits != 0 {
			t.Errorf("test %d, %s, bit buffer not drained: got %d, want 0", i, ne, bw.numBits)
		}
		if bw.cntBuf != 0 {
			t.Errorf("test %d, %s, byte buffer not drained: got %d, want 0", i, ne, bw.cntBuf)
		}
		if ofs != int64(wr.Len()) {
			t.Errorf("test %d, %s, offset mismatch: got %d, want %d", i, ne, ofs, wr.Len())
		}
		if err != nil {
			t.Errorf("test %d, %s, unexpected flush error: got %v", i, ne, err)
		}

		// Check that output matches expected.
		buf := wr.Bytes()
		if endian {
			buf = reverseBytes(buf)
		}
		if bytes.Compare(buf, testVector) != 0 {
			t.Errorf("test %d, %s, output string mismatch:\ngot  %x\nwant %x", i, ne, buf, testVector)
		}
		i++
	}
}

func TestGenerate(t *testing.T) {
	r := newRand()
	var makeCodes = func(freqs []uint) PrefixCodes {
		codes := make(PrefixCodes, len(freqs))
		for i, j := range r.Perm(len(freqs)) {
			codes[i] = PrefixCode{Sym: uint32(i), Cnt: uint32(freqs[j])}
		}
		codes.SortByCount()
		return codes
	}

	var vectors = []struct {
		maxBits uint // Maximum prefix bit-length (0 to skip GenerateLengths)
		input   PrefixCodes
		valid   bool
	}{{
		maxBits: 15,
		input:   makeCodes([]uint{}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{0}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{5}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{0, 0}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{5, 15}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{1, 1, 2, 4}),
		valid:   true,
	}, {
		maxBits: 2,
		input:   makeCodes([]uint{1, 1, 2, 4}),
		valid:   true,
	}, {
		maxBits: 7,
		input:   makeCodes([]uint{100, 101, 102, 103}),
		valid:   true,
	}, {
		maxBits: 10,
		input:   makeCodes([]uint{2, 2, 2, 2, 5, 5, 5}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{1, 2, 3, 4, 5, 6, 7, 8, 9}),
		valid:   true,
	}, {
		maxBits: 15,
		input:   makeCodes([]uint{0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}),
		valid:   true,
	}, {
		maxBits: 7,
		input:   makeCodes([]uint{0, 0, 2, 3, 4, 4, 4, 5, 5, 6, 6, 7, 7, 9, 10, 11, 13, 15}),
		valid:   true,
	}, {
		maxBits: 20,
		input:   makeCodes([]uint{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}),
		valid:   true,
	}, {
		maxBits: 12,
		input:   makeCodes([]uint{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}),
		valid:   true,
	}, {
		maxBits: 15,
		input: makeCodes([]uint{
			1, 1, 1, 1, 1, 2, 2, 3, 3, 4, 4, 4, 4, 6, 6, 7, 7, 8, 8, 9, 9, 11, 11,
			11, 11, 14, 15, 15, 17, 17, 18, 19, 19, 19, 20, 20, 21, 24, 26, 26, 31,
			32, 34, 35, 38, 40, 43, 47, 48, 50, 59, 62, 63, 75, 78, 79, 85, 86, 97,
			100, 100, 102, 114, 119, 128, 128, 139, 153, 166, 170, 174, 182, 184,
			185, 186, 205, 325, 536, 948, 1610, 2555, 2628, 3741,
		}),
		valid: true,
	}, {
		// Input counts are not sorted in ascending order.
		maxBits: 15,
		input: []PrefixCode{
			{Sym: 0, Cnt: 3},
			{Sym: 1, Cnt: 2},
			{Sym: 2, Cnt: 1},
		},
		valid: false,
	}, {
		// Input symbols are not sorted in ascending order.
		maxBits: 0,
		input: []PrefixCode{
			{Sym: 2, Len: 1},
			{Sym: 1, Len: 2},
			{Sym: 0, Len: 2},
		},
		valid: false,
	}, {
		// Input symbols are not unique.
		maxBits: 0,
		input: []PrefixCode{
			{Sym: 5, Len: 1},
			{Sym: 5, Len: 1},
		},
		valid: false,
	}, {
		// Invalid small tree.
		maxBits: 0,
		input: []PrefixCode{
			{Sym: 0, Len: 500},
		},
		valid: false,
	}, {
		// Some bit-length is too short.
		maxBits: 0,
		input: []PrefixCode{
			{Sym: 0, Len: 1},
			{Sym: 1, Len: 2},
			{Sym: 2, Len: 0},
		},
		valid: false,
	}, {
		// Under-subscribed tree.
		maxBits: 0,
		input: []PrefixCode{
			{Sym: 0, Len: 3},
			{Sym: 1, Len: 4},
			{Sym: 2, Len: 3},
		},
		valid: false,
	}, {
		// Over-subscribed tree.
		maxBits: 0,
		input: []PrefixCode{
			{Sym: 0, Len: 1},
			{Sym: 1, Len: 3},
			{Sym: 2, Len: 4},
			{Sym: 3, Len: 3},
			{Sym: 4, Len: 2},
		},
		valid: false,
	}}

	for i, v := range vectors {
		var sum uint32
		var maxLen uint
		var lens []int
		var symBits [valueBits + 1]uint

		codes := v.input
		if v.maxBits == 0 {
			goto genPrefixes
		}

		if err := GenerateLengths(codes, v.maxBits); err != nil {
			if v.valid {
				t.Errorf("test %d, unexpected failure", i)
			}
			continue
		}

		for _, c := range codes {
			if maxLen < uint(c.Len) {
				maxLen = uint(c.Len)
			}
			symBits[c.Len]++
			lens = append(lens, int(c.Len))
			sum += c.Cnt
		}

		if !codes.checkLengths() {
			t.Errorf("test %d, incomplete tree generated", i)
		}
		if !sort.IsSorted(sort.Reverse(sort.IntSlice(lens))) {
			t.Errorf("test %d, bit-lengths are not sorted:\ngot %v", i, lens)
		}
		if maxLen > v.maxBits {
			t.Errorf("test %d, max bit-length exceeded: %d not in 1..%d", i, maxLen, v.maxBits)
		}

		// The whole point of prefix encoding is that the resulting bit-lengths
		// produce an encoding with close to ideal entropy. Thus, compute the
		// best-case entropy and check that we're not too far from it.
		if len(codes) >= 4 && sum > 0 {
			var worst, got, best float64
			worst = math.Log2(float64(len(codes)))
			got = float64(codes.Length()) / float64(sum)
			for _, c := range codes {
				if c.Cnt > 0 {
					p := float64(c.Cnt) / float64(sum)
					best += -(p * math.Log2(p))
				}
			}

			if got > worst {
				t.Errorf("test %d, actual entropy worst than worst-case: %0.3f > %0.3f", i, got, worst)
			}
			if got < best {
				t.Errorf("test %d, actual entropy better than best-case: %0.3f < %0.3f", i, got, best)
			}
			if got > 1.15*best {
				t.Errorf("test %d, actual entropy too high: %0.3f > %0.3f", i, got, 1.15*best)
			}
		}
		codes.SortBySymbol()

	genPrefixes:
		if err := GeneratePrefixes(codes); err != nil {
			if v.valid {
				t.Errorf("test %d, unexpected failure", i)
			}
			continue
		}

		if !codes.checkPrefixes() {
			t.Errorf("test %d, tree with non-unique prefixes generated", i)
		}
		if !codes.checkCanonical() {
			t.Errorf("test %d, tree with non-canonical prefixes generated", i)
		}
		if !v.valid {
			t.Errorf("test %d, unexpected success", i)
		}
	}
}

func TestPrefix(t *testing.T) {
	var makeCodes = func(freqs []uint) PrefixCodes {
		codes := make(PrefixCodes, len(freqs))
		for i, n := range freqs {
			codes[i] = PrefixCode{Sym: uint32(i), Cnt: uint32(n)}
		}
		codes.SortByCount()
		if err := GenerateLengths(codes, 15); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		codes.SortBySymbol()
		if err := GeneratePrefixes(codes); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return codes
	}

	var vectors = []struct {
		codes PrefixCodes
	}{{
		codes: makeCodes([]uint{}),
	}, {
		codes: makeCodes([]uint{0}),
	}, {
		codes: makeCodes([]uint{2, 4, 3, 2, 2, 4}),
	}, {
		codes: makeCodes([]uint{2, 2, 2, 2, 5, 5, 5}),
	}, {
		codes: makeCodes([]uint{100, 101, 102, 103}),
	}, {
		codes: makeCodes([]uint{
			1, 1, 1, 1, 1, 2, 2, 2, 3, 4, 5, 6, 6, 7, 8, 9, 9, 10, 11, 11, 12, 12,
			14, 15, 15, 16, 18, 18, 19, 19, 20, 20, 20, 25, 25, 27, 29, 31, 32, 35,
			39, 44, 47, 52, 60, 62, 71, 73, 74, 82, 86, 97, 98, 103, 108, 110, 112,
			125, 130, 142, 154, 155, 160, 185, 198, 204, 204, 219, 222, 259, 262,
			292, 296, 302, 334, 434, 450, 679, 697, 1032, 1441, 1888, 1892, 2188,
		}),
	}, {
		codes: testCodes,
	}, {
		// Sparsely allocated symbols.
		codes: []PrefixCode{
			{Sym: 16, Val: 0, Len: 1},
			{Sym: 32, Val: 1, Len: 2},
			{Sym: 64, Val: 3, Len: 3},
			{Sym: 128, Val: 7, Len: 3},
		},
	}, {
		// Large number of symbols.
		codes: func() PrefixCodes {
			freqs := make([]uint, 4096)
			for i := range freqs {
				freqs[i] = uint(i)
			}
			return makeCodes(freqs)
		}(),
	}, {
		// Max RLE codes from Brotli.
		codes: func() (codes PrefixCodes) {
			codes = PrefixCodes{{Sym: 0, Val: 0, Len: 1}}
			for i := uint32(0); i < 16; i++ {
				var code = PrefixCode{Sym: i + 1, Val: i<<1 | 1, Len: 5}
				codes = append(codes, code)
			}
			return codes
		}(),
	}, {
		// Window bits codes from Brotli.
		codes: func() (codes PrefixCodes) {
			for i := uint32(9); i <= 24; i++ {
				var code PrefixCode
				switch {
				case i == 16:
					code = PrefixCode{Sym: i, Val: (i-16)<<0 | 0, Len: 1} // Symbols: 16
				case i > 17:
					code = PrefixCode{Sym: i, Val: (i-17)<<1 | 1, Len: 4} // Symbols: 18..24
				case i < 17:
					code = PrefixCode{Sym: i, Val: (i-8)<<4 | 1, Len: 7} // Symbols: 9..15
				default:
					code = PrefixCode{Sym: i, Val: (i-17)<<4 | 1, Len: 7} // Symbols: 17
				}
				codes = append(codes, code)
			}
			codes[0].Sym = 0
			return codes
		}(),
	}, {
		// Count codes from Brotli.
		codes: func() (codes PrefixCodes) {
			codes = PrefixCodes{{Sym: 1, Val: 0, Len: 1}}
			c := codes[len(codes)-1]
			for i := uint32(0); i < 8; i++ {
				for j := uint32(0); j < 1<<i; j++ {
					c.Sym = c.Sym + 1
					c.Val = j<<4 | i<<1 | 1
					c.Len = uint32(i + 4)
					codes = append(codes, c)
				}
			}
			return codes
		}(),
	}, {
		// Fixed literal codes from DEFLATE.
		codes: func() (codes PrefixCodes) {
			for i := 0; i < 144; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 8})
			}
			for i := 144; i < 256; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 9})
			}
			for i := 256; i < 280; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 7})
			}
			for i := 280; i < 288; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 8})
			}
			if err := GeneratePrefixes(codes); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			return codes
		}(),
	}, {
		// Fixed distance codes from DEFLATE.
		codes: func() (codes PrefixCodes) {
			for i := 0; i < 32; i++ {
				codes = append(codes, PrefixCode{Sym: uint32(i), Len: 5})
			}
			if err := GeneratePrefixes(codes); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			return codes
		}(),
	}}

	for i, v := range vectors {
		// Generate an arbitrary prefix Decoder and Encoder.
		var pd Decoder
		var pe Encoder
		pd.Init(v.codes)
		pe.Init(v.codes)
		if len(v.codes) == 0 {
			continue
		}

		// Create an arbitrary list of symbols to encode.
		r := newRand()
		syms := make([]uint, 1000)
		for i := range syms {
			syms[i] = uint(v.codes[r.Intn(len(v.codes))].Sym)
		}

		// Setup a Reader and Writer.
		var buf bytes.Buffer
		var rd Reader
		var wr Writer
		rdwr := struct {
			io.Reader
			io.ByteReader
			io.Writer
		}{&buf, &buf, &buf}
		rd.Init(rdwr, false)
		wr.Init(rdwr, false)

		// Write some symbols.
		for _, sym := range syms {
			ok := wr.TryWriteSymbol(sym, &pe)
			if !ok {
				wr.WriteSymbol(sym, &pe)
			}
		}
		wr.WritePads(0)
		if _, err := wr.Flush(); err != nil {
			t.Errorf("test %d, unexpected Writer error: %v", i, err)
		}

		// Verify some Writer statistics.
		if wr.offset != int64(buf.Len()) {
			t.Errorf("test %d, offset mismatch: got %d, want %d", i, wr.offset, buf.Len())
		}
		if wr.numBits != 0 {
			t.Errorf("test %d, residual bits remaining: got %d, want 0", i, wr.numBits)
		}
		if wr.cntBuf != 0 {
			t.Errorf("test %d, residual bytes remaining: got %d, want 0", i, wr.cntBuf)
		}

		// Read some symbols.
		for i := range syms {
			sym, ok := rd.TryReadSymbol(&pd)
			if !ok {
				sym = rd.ReadSymbol(&pd)
			}
			if sym != syms[i] {
				t.Errorf("test %d, read back wrong symbol: got %d, want %d", i, sym, syms[i])
			}
			if rd.numBits >= 8 {
				t.Errorf("test %d, residual bits remaining: got %d, want < 8", i, rd.numBits)
			}
		}
		pads := rd.ReadPads()
		if _, err := rd.Flush(); err != nil {
			t.Errorf("test %d, unexpected Reader error: %v", i, err)
		}

		// Verify some Reader statistics.
		if pads != 0 {
			t.Errorf("test %d, unexpected padding bits: got %d, want 0", i, pads)
		}
		if rd.numBits != 0 {
			t.Errorf("test %d, residual bits remaining: got %d, want 0", i, rd.numBits)
		}
		if rd.offset != wr.offset {
			t.Errorf("test %d, offset mismatch: got %d, want %d", i, rd.offset, wr.offset)
		}
	}
}

func TestRange(t *testing.T) {
	var vectors = []struct {
		input RangeCodes
		valid bool
	}{{
		input: RangeCodes{},
		valid: false,
	}, {
		input: RangeCodes{{5, 2}, {10, 5}}, // Gap in-between
		valid: false,
	}, {
		input: RangeCodes{{5, 20}, {7, 5}}, // All-encompassing overlap
		valid: false,
	}, {
		input: RangeCodes{{7, 5}, {5, 2}}, // Out-of-order
		valid: false,
	}, {
		input: RangeCodes{{5, 10}, {6, 11}}, // Forward-overlap is okay
		valid: true,
	}, {
		input: testRanges,
		valid: true,
	}, {
		input: MakeRangeCodes(0, []uint{
			0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 12, 14, 24,
		}),
		valid: true,
	}, {
		input: MakeRangeCodes(2, []uint{
			0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 7, 8, 9, 10, 24,
		}),
		valid: true,
	}, {
		input: MakeRangeCodes(1, []uint{
			2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 7, 8, 9, 10, 11, 12, 13, 24,
		}),
		valid: true,
	}, {
		input: MakeRangeCodes(2, []uint{
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		}),
		valid: true,
	}, {
		input: append(MakeRangeCodes(3, []uint{
			0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5,
		}), RangeCode{Base: 258, Len: 0}),
		valid: true,
	}, {
		input: MakeRangeCodes(1, []uint{
			0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13,
		}),
		valid: true,
	}}

	r := newRand()
	for i, v := range vectors {
		if valid := v.input.checkValid(); valid != v.valid {
			t.Errorf("test %d, validity mismatch: got %v, want %v", i, valid, v.valid)
		}
		if !v.valid {
			continue // No point further testing invalid ranges
		}

		var re RangeEncoder
		re.Init(v.input)

		for _, rc := range v.input {
			offset := rc.Base + uint32(r.Intn(int(rc.End()-rc.Base)))
			sym := re.Encode(uint(offset))
			if int(sym) >= len(v.input) {
				t.Errorf("test %d, invalid symbol: re.Encode(%d) = %d", i, offset, sym)
			}
			rc := v.input[sym]
			if offset < rc.Base || offset >= rc.End() {
				t.Errorf("test %d, symbol not in range: %d not in %d..%d", i, offset, rc.Base, rc.End()-1)
			}
		}
	}
}
