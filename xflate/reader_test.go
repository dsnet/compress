// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package xflate

import (
	"bytes"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/dsnet/compress/internal/testutil"
)

type countReadSeeker struct {
	io.ReadSeeker
	N int64
}

func (rs *countReadSeeker) Read(buf []byte) (int, error) {
	n, err := rs.ReadSeeker.Read(buf)
	rs.N += int64(n)
	return n, err
}

func TestReader(t *testing.T) {
	var dh = testutil.MustDecodeHex

	var vectors = []struct {
		desc   string // Description of the test
		input  []byte // Input test string
		output []byte // Expected output string
		err    error  // Expected error
	}{{
		desc: "empty string",
		err:  ErrCorrupt,
	}, {
		desc: "empty stream",
		input: dh("" +
			"0d008705000048c82a51e8ff37dbf1",
		),
	}, {
		desc: "empty stream with empty chunk",
		input: dh("" +
			"000000ffff000000ffff34c086050020916cb2a50bd20369da192deaff3bda05" +
			"f81dc08605002021ab44219b4aff7fd6de3bf8",
		),
	}, {
		desc: "empty stream with empty index",
		input: dh("" +
			"04c086050020191d53a1a508c9e8ff5bda7bf815c08605002021ab44219ba2ff" +
			"2f6bef5df8",
		),
	}, {
		desc: "empty stream with multiple empty chunks",
		input: dh("" +
			"000000ffff000000ffff000000ffff148086058044655366e3817441ba205d50" +
			"4a83348c445ddcde7b6ffc15c08605002021ab44a103aaff2f6bef5df8",
		),
	}, {
		desc: "empty stream with multiple empty chunks, with final bit",
		input: dh("" +
			"000000ffff010000ffff000000ffff148086058044655366e3817441ba205d50" +
			"4a83348c445ddcde7b6ffc15c08605002021ab44a103aaff2f6bef5df8",
		),
		err: ErrCorrupt,
	}, {
		desc: "empty stream with multiple empty indexes",
		input: dh("" +
			"04c086050020191d53a1a508c9e8ff5bda7bf83cc08605002019293a24a55464" +
			"a585faff9bf600f804c08605002019493a2494d050560afd7f4c7bfb25008705" +
			"000048c82a51e880f4ff834df0",
		),
	}, {
		desc: "3k zeros, 1KiB chunks",
		input: dh("" +
			"621805a360148c5800000000ffff621805a360148c5800000000ffff621805a3" +
			"60140c3900000000ffff1c8086058044642b3bc9aa3464540784acea809055d9" +
			"9586dd5492446555a7b607fc0d008705000048c82a51c81ea1ff0f6cf2",
		),
		output: make([]byte, 3000),
	}, {
		desc: "quick brown fox",
		input: dh("" +
			"2ac94855282ccd4cce06000000ffff52482aca2fcf5348cbaf00000000ffff00" +
			"0000ffff52c82acd2d484d51c82f4b2d5228c94805000000ffff248086058044" +
			"6553762a0ad14211d207253b234546a1528ad4d3edbd0bfc52c849acaa5448c9" +
			"4f07000000ffff2c8086058044a281ec8611190d23b21221ca0851fdafbdf7de" +
			"05fc1dc08605002021ab44219b52ff7fd6de3bf8",
		),
		output: []byte("the quick brown fox jumped over the lazy dog"),
	}, {
		desc: "alphabet",
		input: dh("" +
			"4a4c4a4e494d4bcfc8cccacec9cdcb2f282c2a2e292d2bafa8ac02000000ffff" +
			"048086058044b2e98190b285148a844a0b95a4f7db7bef3dfc15c08605002021" +
			"ab44219ba8ff2f6bef5df8",
		),
		output: []byte("abcdefghijklmnopqrstuvwxyz"),
	}, {
		desc:  "garbage footer",
		input: dh("5174453181b67484bf6de23a608876f8b7f44c77"),
		err:   ErrCorrupt,
	}, {
		desc:  "corrupt meta footer",
		input: dh("1d008705000048ca2c50e8ff3bdbf0"),
		err:   ErrCorrupt,
	}, {
		desc:  "trailing meta data in footer",
		input: dh("0d008705000048c82a51e8ff37dbf1deadcafe"),
		err:   ErrCorrupt,
	}, {
		desc:  "trailing raw data in footer",
		input: dh("25c086050020a9ac12856ec8284229d4ff0fb527f8"),
		err:   ErrCorrupt,
	}, {
		desc:  "footer using LastMeta",
		input: dh("0c008705000048c82a51e8ff37dbf1"),
		err:   ErrCorrupt,
	}, {
		desc:  "footer without magic",
		input: dh("1d00870500004864a644eaff3bdbf0"),
		err:   ErrCorrupt,
	}, {
		desc:  "footer with VLI overflow",
		input: dh("2d80860580944a458a4abb6e6c9fdbde7bef01fc"),
		err:   ErrCorrupt,
	}, {
		desc: "index using LastStream",
		input: dh("" +
			"05c086050020191d53a1a508c9e8ff5bda7bf815c08605002021ab44219ba2ff" +
			"2f6bef5df8",
		),
		err: ErrCorrupt,
	}, {
		desc: "index with wrong CRC",
		input: dh("" +
			"2cc086050020191d132551320a51ff9fd2de0bf825008705000048c82a51e880" +
			"f4ff834df0",
		),
		err: ErrCorrupt,
	}, {
		desc: "corrupt meta index",
		input: dh("" +
			"04c086050020191d53a1a518c9e8ff5bda7bf815c08605002021ab44219ba2ff" +
			"2f6bef5df8",
		),
		err: ErrCorrupt,
	}, {
		desc: "index with VLI overflow",
		input: dh("" +
			"048086058094e8c6f6de7b531215458a840e6deffc15c08605002021ab44219b" +
			"a4ff2f6bef5df8",
		),
		err: ErrCorrupt,
	}, {
		desc: "trailing meta data in index",
		input: dh("" +
			"34c086050020291d53a1a508c908a16414a2fe3fa205f81dc08605002021ab44" +
			"219b4aff7fd6de3bf8",
		),
		err: ErrCorrupt,
	}, {
		desc: "trailing raw data in index",
		input: dh("" +
			"04c086050020191d53a1a508c9e8ff5bda7bf862616405c08605002021ab4421" +
			"7b94febfacbd77f9",
		),
		err: ErrCorrupt,
	}, {
		desc: "index total size is wrong",
		input: dh("" +
			"000000ffff14c086050020916cb2d505e983840aa12592faff8c76f81dc08605" +
			"002021ab44219b4aff7fd6de3bf8",
		),
		err: ErrCorrupt,
	}, {
		desc: "index with compressed chunk size of zero",
		input: dh("" +
			"000000ffff04c086050020916cb2e9848e8894a2a441fd7f457bf905c0860500" +
			"2021ab44217b94febfacbd77f9",
		),
		err: ErrCorrupt,
	}, {
		desc: "index with numeric overflow on sizes",
		input: dh("" +
			"000000ffff000000ffff0c40860552a43db4a53dcf6b97b47724641589a84e69" +
			"efbdf7de7b4ffe1dc08605002021ab44219b54ff7fd6de3bf8",
		),
		err: ErrCorrupt,
	}, {
		desc: "empty chunk without sync marker",
		input: dh("" +
			"000000ffff020820800004c086050020a1ec919d1e4817a40b421269a3a8ff1f" +
			"68fa2d008705000048c82a51e881faffc126f0",
		),
		err: ErrCorrupt,
	}, {
		desc: "chunk without sync marker",
		input: dh("" +
			"000000ffff000200fdff486902082080000cc086050020a1ec91193232d30965" +
			"652b2b221125f5ff1eedf805c08605002021ab44217ba4febfacbd77f9",
		),
		output: []byte("Hi"),
		err:    ErrCorrupt,
	}, {
		desc: "chunk with wrong sizes",
		input: dh("" +
			"000000ffff000200fdff4869000000ffff2c8086058084b2476608d9e98432b2" +
			"15252a958a92eaeef6de7b07fc15c08605002021ab44a103aaff2f6bef5df8",
		),
		output: []byte("Hi"),
		err:    ErrCorrupt,
	}, {
		desc: "size overflow across multiple indexes",
		input: dh("" +
			"000000ffff0c8086058094b487b6b4ce4b5ae7150d49d124195dd29efc000000" +
			"ffff000000ffff24808605808432cac84e4676ba2059d9914a4a29259a8fb7f7" +
			"de0bfc15c08605002021ab44a103aaff2f6bef5df8",
		),
		err: ErrCorrupt,
	}, {
		desc: "raw chunk with final bit and bad size",
		input: dh("" +
			"010900f6ff0000ffff248086058044b2c98e8cc8888cc828ed9d284afa7fb4f7" +
			"de0bfc05c08605002021ab44217ba4febfacbd77f9",
		),
		output: dh("0000ffff010000ffff"),
		// TODO(dsnet): The Reader incorrectly believes that this is valid.
		// The chunk has a final raw block with a specified size of 9, but only
		// has 4 bytes following it (0000ffff to fool the sync check).
		// Since the decompressor would expect an additional 5 bytes, this is
		// satisfied by the fact that the chunkReader appends the endBlock
		// sequence (010000ffff) to every chunk. This really difficult to fix
		// without low-level details about the DEFLATE stream.
		err: nil, // ErrCorrupt,
	}}

	for i, v := range vectors {
		var xr *Reader
		var err error
		var buf []byte

		xr, err = NewReader(bytes.NewReader(v.input), nil)
		if err != nil {
			goto done
		}

		buf, err = ioutil.ReadAll(xr)
		if err != nil {
			goto done
		}

	done:
		if err != v.err {
			t.Errorf("test %d (%s), mismatching error: got %v, want %v", i, v.desc, err, v.err)
		}
		if !bytes.Equal(buf, v.output) && err == nil {
			t.Errorf("test %d (%s), mismatching output:\ngot  %q\nwant %q", i, v.desc, buf, v.output)
		}
	}
}

func TestReaderReset(t *testing.T) {
	var (
		empty   = testutil.MustDecodeHex("0d008705000048c82a51e8ff37dbf1")
		badSize = testutil.MustDecodeHex("" +
			"4a4c4a4e494d4bcfc8cccacec9cdcb2f282c2a2e292d2bafa8ac02000000ffff" +
			"3c8086058084b2e981acd0203b2b34884a834a2a91d2ededbd7701fc15c08605" +
			"002021ab44a103aaff2f6bef5df8",
		)
		badData = testutil.MustDecodeHex("" +
			"4a4c4a4e494d4bcfc8cccacec9cdcb2f282c2a2e292d2baf000002000000ffff" +
			"048086058044b2e98190b285148a844a0b95a4f7db7bef3dfc15c08605002021" +
			"ab44219ba8ff2f6bef5df8",
		)
	)

	// Test Reader for idempotent Close.
	xr := new(Reader)
	if err := xr.Reset(bytes.NewReader(empty)); err != nil {
		t.Fatalf("unexpected error: Reset() = %v", err)
	}
	buf, err := ioutil.ReadAll(xr)
	if err != nil {
		t.Fatalf("unexpected error: ReadAll() = %v", err)
	}
	if len(buf) > 0 {
		t.Fatalf("unexpected output data: ReadAll() = %q, want nil", buf)
	}
	if err := xr.Close(); err != nil {
		t.Fatalf("unexpected error: Close() = %v", err)
	}
	if err := xr.Close(); err != nil {
		t.Fatalf("unexpected error: Close() = %v", err)
	}
	if _, err := ioutil.ReadAll(xr); err != errClosed {
		t.Fatalf("mismatching error: ReadAll() = %v, want %v", err, errClosed)
	}

	// Test Reset on garbage data.
	rd := bytes.NewReader(append([]byte("garbage"), empty...))
	if err := xr.Reset(rd); err != ErrCorrupt {
		t.Fatalf("mismatching error: Reset() = %v, want %v", err, ErrCorrupt)
	}
	if _, err := xr.Seek(0, os.SEEK_SET); err != ErrCorrupt {
		t.Fatalf("mismatching error: Seek() = %v, want %v", err, ErrCorrupt)
	}
	if err := xr.Close(); err != ErrCorrupt {
		t.Fatalf("mismatching error: Close() = %v, want %v", err, ErrCorrupt)
	}

	// Test Reset on corrupt data in discard section.
	for i, v := range [][]byte{badData, badSize} {
		if err := xr.Reset(bytes.NewReader(v)); err != nil {
			t.Fatalf("test %d, unexpected error: Reset() = %v", i, err)
		}
		if _, err := xr.Seek(-1, os.SEEK_END); err != nil {
			t.Fatalf("test %d, unexpected error: Seek() = %v", i, err)
		}
		_, err = ioutil.ReadAll(xr)
		if err != ErrCorrupt {
			t.Fatalf("test %d, mismatching error: ReadAll() = %v, want %v", i, err, ErrCorrupt)
		}
	}
}

func TestReaderSeek(t *testing.T) {
	rand := rand.New(rand.NewSource(0))

	var twain = testutil.MustLoadFile("../testdata/twain.txt", -1)

	// Generate compressed version of input file.
	var buf bytes.Buffer
	xw, err := NewWriter(&buf, &WriterConfig{ChunkSize: 1 << 10})
	if err != nil {
		t.Fatalf("unexpected error: NewWriter() = %v", err)
	}
	if _, err := xw.Write(twain); err != nil {
		t.Fatalf("unexpected error: Write() = %v", err)
	}
	if err := xw.Close(); err != nil {
		t.Fatalf("unexpected error: Close() = %v", err)
	}

	// Read the compressed file.
	rs := &countReadSeeker{ReadSeeker: bytes.NewReader(buf.Bytes())}
	xr, err := NewReader(rs, nil)
	if err != nil {
		t.Fatalf("unexpected error: NewReader() = %v", err)
	}

	// As a heuristic, make sure we are not reading too much data.
	if thres := int64(len(twain) / 100); rs.N > thres {
		t.Fatalf("read more data than expected: %d > %d", rs.N, thres)
	}
	rs.N = 0 // Reset the read count

	// Generate list of seek commands to try.
	type seekCommand struct {
		length int   // Number of bytes to read
		offset int64 // Seek to this offset
		whence int   // Whence value to use
	}
	var vectors = []seekCommand{
		{length: 40, offset: int64(len(twain)) - 1, whence: os.SEEK_SET},
		{length: 40, offset: int64(len(twain)), whence: os.SEEK_SET},
		{length: 40, offset: int64(len(twain)) + 1, whence: os.SEEK_SET},
		{length: 40, offset: math.MaxInt64, whence: os.SEEK_SET},
		{length: 0, offset: 0, whence: os.SEEK_CUR},
		{length: 13, offset: 15, whence: os.SEEK_SET},
		{length: 32, offset: 23, whence: os.SEEK_CUR},
		{length: 32, offset: -23, whence: os.SEEK_CUR},
		{length: 13, offset: -15, whence: os.SEEK_SET},
		{length: 100, offset: -15, whence: os.SEEK_END},
		{length: 0, offset: 0, whence: os.SEEK_CUR},
		{length: 0, offset: 0, whence: os.SEEK_CUR},
		{length: 32, offset: -34, whence: os.SEEK_CUR},
		{length: 32, offset: -34, whence: os.SEEK_CUR},
		{length: 2000, offset: 53, whence: os.SEEK_SET},
		{length: 2000, offset: int64(len(twain)) - 1000, whence: os.SEEK_SET},
		{length: 0, offset: 0, whence: os.SEEK_CUR},
		{length: 100, offset: -int64(len(twain)), whence: os.SEEK_END},
		{length: 100, offset: -int64(len(twain)) - 1, whence: os.SEEK_END},
		{length: 0, offset: 0, whence: os.SEEK_SET},
		{length: 10, offset: 10, whence: os.SEEK_CUR},
		{length: 10, offset: 10, whence: os.SEEK_CUR},
		{length: 10, offset: 10, whence: os.SEEK_CUR},
		{length: 10, offset: 10, whence: os.SEEK_CUR},
		{length: 0, offset: 0, whence: -1},
	}

	// Add random values to seek list.
	for i := 0; i < 100; i++ {
		length, offset := rand.Intn(1<<11), rand.Int63n(int64(len(twain)))
		if length+int(offset) <= len(twain) {
			vectors = append(vectors, seekCommand{length, offset, os.SEEK_SET})
		}
	}

	// Read in reverse.
	vectors = append(vectors, seekCommand{0, 0, os.SEEK_END})
	for pos := int64(len(twain)); pos > 0; {
		n := int64(rand.Intn(1 << 11))
		if n > pos {
			n = pos
		}
		pos -= n
		vectors = append(vectors, seekCommand{int(n), pos, os.SEEK_SET})
	}

	// Execute all seek commands.
	var pos, totalLength int64
	for i, v := range vectors {
		// Emulate Seek logic.
		var wantPos int64
		switch v.whence {
		case os.SEEK_SET:
			wantPos = v.offset
		case os.SEEK_CUR:
			wantPos = v.offset + pos
		case os.SEEK_END:
			wantPos = v.offset + int64(len(twain))
		default:
			wantPos = -1
		}

		// Perform actually (short-circuit if seek fails).
		wantFail := bool(wantPos < 0)
		gotPos, err := xr.Seek(v.offset, v.whence)
		if gotFail := bool(err != nil); gotFail != wantFail {
			if gotFail {
				t.Fatalf("test %d, unexpected failure: Seek(%d, %d) = (%d, %v)", i, v.offset, v.whence, pos, err)
			} else {
				t.Fatalf("test %d, unexpected success: Seek(%d, %d) = (%d, nil)", i, v.offset, v.whence, pos)
			}
		}
		if wantFail {
			continue
		}
		if gotPos != wantPos {
			t.Fatalf("test %d, offset mismatch: got %d, want %d", i, gotPos, wantPos)
		}

		// Read and verify some length of bytes.
		var want []byte
		if wantPos < int64(len(twain)) {
			want = twain[wantPos:]
		}
		if len(want) > v.length {
			want = want[:v.length]
		}
		got, err := ioutil.ReadAll(io.LimitReader(xr, int64(v.length)))
		if err != nil {
			t.Fatalf("test %v, unexpected error: ReadAll() = %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("test %v, mismatching output:\ngot  %q\nwant %q", i, got, want)
		}

		pos = gotPos + int64(len(got))
		totalLength += int64(v.length)
	}

	// As a heuristic, make sure we are not reading too much data.
	if thres := 2 * totalLength; rs.N > thres {
		t.Fatalf("read more data than expected: %d > %d", rs.N, thres)
	}
}

// BenchmarkWriter benchmarks the overhead of the XFLATE format over DEFLATE.
// Thus, it intentionally uses a very small chunk size with no compression.
// This benchmark reads the input file in reverse to excite poor behavior.
func BenchmarkReader(b *testing.B) {
	rand := rand.New(rand.NewSource(0))
	twain := testutil.MustLoadFile("../testdata/twain.txt", -1)
	bb := bytes.NewBuffer(make([]byte, 0, 2*len(twain)))
	xr := new(Reader)
	lr := new(io.LimitedReader)

	xw, _ := NewWriter(bb, &WriterConfig{Level: NoCompression, ChunkSize: 1 << 10})
	xw.Write(twain)
	xw.Close()

	b.ReportAllocs()
	b.SetBytes(int64(len(twain)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rand.Seed(0)
		rd := bytes.NewReader(bb.Bytes())
		if err := xr.Reset(rd); err != nil {
			b.Fatalf("unexpected error: Reset() = %v", err)
		}

		// Read sections of the input in reverse.
		for pos := int64(len(twain)); pos > 0; {
			// Random section size.
			n := int64(rand.Intn(1 << 11))
			if n > pos {
				n = pos
			}
			pos -= n

			// Read the given section.
			if _, err := xr.Seek(pos, os.SEEK_SET); err != nil {
				b.Fatalf("unexpected error: Seek() = %v", err)
			}
			*lr = io.LimitedReader{R: xr, N: n}
			if _, err := io.Copy(ioutil.Discard, lr); err != nil {
				b.Fatalf("unexpected error: Copy() = %v", err)
			}
		}
	}
}
