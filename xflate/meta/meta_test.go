// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package meta

import "io"
import "io/ioutil"
import "bytes"
import "math"
import "math/big"
import "math/rand"
import "compress/flate"
import "github.com/dsnet/golib/bits"
import "github.com/stretchr/testify/assert"
import "testing"

// Helper test function that converts any empty byte slice to the nil slice so
// that equality checks work out fine.
func nb(buf []byte) []byte {
	if len(buf) == 0 {
		return nil
	}
	return buf
}

// Helper test function to deterministically generate pseudo-random data.
func randBytes(cnt int) (buf []byte) {
	rand := rand.New(rand.NewSource(0))
	for len(buf) < cnt {
		buf = append(buf, byte(rand.Int()&0xff))
	}
	return buf
}

func testBackwardCompatibility(t *testing.T, b []byte) {
	// Works only on Go 1.5 and above due to a bug in Go's flate implementation.
	// See https://github.com/golang/go/issues/11030.
	//
	// The following const holds a valid compressed string that uses an empty
	// HDistTree to trigger the bug before performing the backwards
	// compatibility test below.
	const emptyDistBlock = "\x05\xc0\x07\x06\x00\x00\x00\x80\x40\x0f\xff\x37\xa0\xca"
	zd := flate.NewReader(bytes.NewReader([]byte(emptyDistBlock)))
	if _, err := ioutil.ReadAll(zd); err != nil {
		t.Fatal("Empty HDistTree bug found in compress/flate, please use Go 1.5 and above")
	}

	// Append last stream block that just contains the string "test\n".
	const rawTestBlock = "\x01\x05\x00\xfa\xfftest\n"
	zd = flate.NewReader(bytes.NewBuffer([]byte(string(b) + rawTestBlock)))
	data, err := ioutil.ReadAll(zd)
	assert.Nil(t, err)
	assert.Equal(t, "test\n", string(data))
}

func TestInterfaces(t *testing.T) {
	assert.Implements(t, (*io.Reader)(nil), new(Reader))
	assert.Implements(t, (*io.ByteReader)(nil), new(Reader))
	assert.Implements(t, (*io.Writer)(nil), new(Writer))
	assert.Implements(t, (*io.ByteWriter)(nil), new(Writer))
}

func TestFuzz(t *testing.T) {
	type X struct {
		buf  []byte
		cnt  int
		last LastMode
	}
	expects := []X{}
	buf := bytes.NewBuffer(nil)
	mw := new(Writer)

	// Encode test.
	rand := rand.New(rand.NewSource(0))
	for numBytes := MinRawBytes; numBytes <= MaxRawBytes; numBytes++ {
		numBits := numBytes * 8
		for zeros := 0; zeros <= numBits; zeros++ {
			ones := numBits - zeros
			huffLen := mw.computeHuffLen(zeros, ones)
			assert.True(t, huffLen > 0 || numBytes > EnsureRawBytes)
			if huffLen == 0 {
				continue
			}

			bb := bits.NewBuffer(nil)
			for _, x := range rand.Perm(numBits) {
				bb.WriteBit(x >= zeros)
			}
			for _, l := range []LastMode{LastNil, LastMeta} {
				cnt, err := mw.encodeBlock(buf, bb.Bytes(), l)
				assert.Nil(t, err)
				expects = append(expects, X{bb.Bytes(), cnt, l})

				// Ensure theoretical limits are upheld.
				assert.True(t, MinEncBytes <= cnt)
				assert.True(t, MaxEncBytes >= cnt)
			}
		}
	}

	testBackwardCompatibility(t, buf.Bytes())

	// Decode test.
	mr := new(Reader)
	for _, x := range expects {
		b, l, n, err := mr.decodeBlock(buf)
		assert.Equal(t, nb(x.buf), nb(b))
		assert.Equal(t, x.last, l)
		assert.Equal(t, x.cnt, n)
		assert.Nil(t, err)
	}
}

func TestRandom(t *testing.T) {
	obuf := bytes.NewBuffer(nil)
	ibuf := bytes.NewBuffer(nil)
	mw := NewWriter(obuf, LastMeta)

	// Encode writer test.
	rand := rand.New(rand.NewSource(0))
	for i := 0; i < 1000; i++ {
		rdat := randBytes(rand.Intn(100))
		ibuf.Write(rdat)
		wrCnt, err := mw.Write(rdat)
		assert.Equal(t, len(rdat), wrCnt)
		assert.Equal(t, int64(obuf.Len()), mw.WriteCount())
		assert.Nil(t, err)
	}
	assert.Nil(t, mw.Close())

	testBackwardCompatibility(t, obuf.Bytes())

	// Meta encoding should be better than 50% on large inputs.
	eff := 100.0 * float64(len(ibuf.Bytes())) / float64(len(obuf.Bytes()))
	assert.True(t, eff > 50.0)

	// Decode reader test.
	mr := NewReader(bytes.NewReader(obuf.Bytes()))
	dat, err := ioutil.ReadAll(mr)
	assert.Equal(t, nb(ibuf.Bytes()), nb(dat))
	assert.Equal(t, LastMeta, mr.LastMarker())
	assert.Equal(t, mw.WriteCount(), mr.ReadCount())
	assert.Equal(t, mw.BlockCount(), mr.BlockCount())
	assert.Nil(t, err)
}

func TestReadWriter(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	data := make([]byte, 4096)

	// Read EOF.
	mr := NewReader(buf)
	_, err := mr.ReadByte()
	assert.True(t, mr.AtEOF())
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, LastNil, mr.LastMarker())
	assert.Equal(t, []byte(nil), nb(mr.BlockData()))
	assert.Equal(t, int64(0), mr.ReadCount())
	assert.Equal(t, int64(0), mr.BlockCount())

	// Read ErrUnexpectedEOF.
	mr = NewReader(bytes.NewBuffer([]byte{0xff}))
	cnt, err := mr.Read(data)
	assert.False(t, mr.AtEOF())
	assert.Equal(t, 0, cnt)
	assert.Equal(t, io.ErrUnexpectedEOF, err)
	assert.Equal(t, LastNil, mr.LastMarker())
	assert.Equal(t, []byte{0xff}, nb(mr.BlockData()))
	assert.Equal(t, int64(1), mr.ReadCount())
	assert.Equal(t, int64(0), mr.BlockCount())

	// Write empty block.
	mw := NewWriter(buf, LastMeta)
	cnt, err = mw.Write(nil)
	assert.Equal(t, 0, cnt)
	assert.Nil(t, err)
	assert.Nil(t, mw.Close())
	assert.True(t, MinEncBytes <= mw.WriteCount() && mw.WriteCount() <= MaxEncBytes)
	assert.Equal(t, int64(1), mw.BlockCount())

	// Read empty block.
	blk := buf.Bytes()
	mr = NewReader(buf)
	_, err = mr.ReadByte()
	assert.True(t, mr.AtEOF())
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, LastMeta, mr.LastMarker())
	assert.Equal(t, blk, nb(mr.BlockData()))
	assert.Equal(t, mw.WriteCount(), mr.ReadCount())
	assert.Equal(t, mw.BlockCount(), mr.BlockCount())

	// Write hello block.
	mw = NewWriter(buf, LastMeta)
	vector1 := "Hello, world!"
	cnt, err = mw.Write([]byte(vector1))
	assert.Equal(t, len(vector1), cnt)
	assert.Nil(t, err)
	assert.Nil(t, mw.Close())
	assert.True(t, MinEncBytes <= mw.WriteCount() && mw.WriteCount() <= MaxEncBytes)
	assert.Equal(t, int64(1), mw.BlockCount())

	// Read hello block.
	blk = buf.Bytes()
	mr = NewReader(buf)
	val, err := mr.ReadByte()
	assert.Equal(t, vector1[0], val)
	assert.Nil(t, err)
	cnt, err = mr.Read(data)
	assert.True(t, mr.AtEOF())
	assert.Equal(t, vector1[1:], string(data[:cnt]))
	assert.Equal(t, len(vector1)-1, cnt)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, LastMeta, mr.LastMarker())
	assert.Equal(t, blk, nb(mr.BlockData()))
	assert.Equal(t, mw.WriteCount(), mr.ReadCount())
	assert.Equal(t, mw.BlockCount(), mr.BlockCount())

	// Write several blocks worth.
	var cnt1, cnt2, cnt3 int
	vector2 := "the quick brown fox jumped over the lazy dog. the kitchen caught on fire. no joke."
	maxNumBlocks := int64(divCeil(len(vector2), EnsureRawBytes))
	minNumBlocks := int64(divCeil(len(vector2), MaxRawBytes))
	mw = NewWriter(buf, LastStream)
	cnt, err = mw.Write([]byte(vector2))
	assert.Equal(t, len(vector2), cnt)
	assert.Nil(t, err)
	assert.Nil(t, mw.Close())
	assert.True(t, minNumBlocks*MinEncBytes <= mw.WriteCount())
	assert.True(t, maxNumBlocks*MaxEncBytes >= mw.WriteCount())
	assert.True(t, minNumBlocks <= mw.BlockCount())
	assert.True(t, maxNumBlocks >= mw.BlockCount())

	// Corrupt last several bytes.
	good := make([]byte, 5)
	copy(good, buf.Bytes()[buf.Len()-len(good):])
	buf.Truncate(buf.Len() - len(good))
	buf.Write([]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee})

	// Read first block successfully.
	mr = NewReader(buf)
	val, err = mr.PeekByte()
	assert.Equal(t, vector2[0], val)
	assert.Nil(t, err)
	cnt1, err = mr.Read(data[:EnsureRawBytes])
	assert.Equal(t, EnsureRawBytes, cnt1)
	assert.Equal(t, vector2[:cnt1], string(data[:cnt1]))
	assert.Nil(t, err)
	val, err = mr.PeekByte()
	assert.Equal(t, vector2[EnsureRawBytes], val)
	assert.Nil(t, err)
	assert.Equal(t, LastNil, mr.LastMarker())
	assert.True(t, mr.ReadCount() <= mw.WriteCount())
	assert.Equal(t, int64(1), mr.BlockCount())

	// Trigger failure on corrupted block.
	cnt2, err = mr.Read(data[cnt1:])
	assert.Equal(t, vector2[:cnt1+cnt2], string(data[:cnt1+cnt2]))
	assert.Implements(t, (*error)(nil), err)
	rdCnt := mr.ReadCount() - int64(len(mr.BlockData()))
	blkCnt := mr.BlockCount()

	// Re-insert good data.
	bad := append(mr.BlockData(), buf.Bytes()...)
	good = append(bad[:len(bad)-len(good)], good...)
	buf.Truncate(0)
	buf.Write(good)

	// Try reading again.
	mr = NewReader(buf)
	val, err = mr.PeekByte()
	assert.Equal(t, vector2[cnt1+cnt2], val)
	assert.Nil(t, err)
	cnt3, err = mr.Read(data)
	assert.Equal(t, len(vector2), cnt1+cnt2+cnt3)
	assert.Equal(t, vector2[cnt1+cnt2:], string(data[:cnt3]))
	assert.Equal(t, io.EOF, err)
	_, err = mr.PeekByte()
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, LastStream, mr.LastMarker())
	assert.Equal(t, mw.WriteCount(), rdCnt+mr.ReadCount())
	assert.Equal(t, mw.BlockCount(), blkCnt+mr.BlockCount())
}

func TestReverseSearch(t *testing.T) {
	// Search random data (not found).
	data := randBytes(4096)
	assert.Equal(t, -1, ReverseSearch(data))

	// Write arbitrary data.
	rand := rand.New(rand.NewSource(0))
	buf := bytes.NewBuffer(nil)
	mw := NewWriter(buf, LastStream)
	for i := 0; i < 4096; i++ {
		mw.Write(randBytes(rand.Intn(MaxEncBytes)))
		mw.flush(LastNil)
	}
	assert.Nil(t, mw.Close())

	// Reverse search all the blocks.
	data = buf.Bytes()
	blkCnt := mw.BlockCount()
	for len(data) > 0 {
		pos := ReverseSearch(data)
		if pos == -1 {
			break
		}
		data = data[:pos]
		blkCnt--
	}
	assert.Equal(t, int64(0), blkCnt)
	assert.Equal(t, 0, len(data))
}

// This test function computes how efficient the meta encoding is at converting
// arbitrary strings into XFLATE's meta encoding format.
func TestEfficiency(t *testing.T) {
	// Print encoding efficiency only in non-short, verbose mode.
	if !testing.Verbose() || testing.Short() {
		t.SkipNow()
	}

	// Compute the combination nCk.
	comb := func(n, k int64) *big.Int {
		val := big.NewInt(1)
		for j := int64(0); j < k; j++ {
			// Compute: val = (val*(n-j)) / (j+1)
			val.Div(new(big.Int).Mul(val, big.NewInt(n-j)), big.NewInt(j+1))
		}
		return val
	}

	// Convert big integer to float64.
	rtof := func(r *big.Rat) float64 { f, _ := r.Float64(); return f }
	itof := func(i *big.Int) float64 { return rtof(new(big.Rat).SetInt(i)) }

	// It would be impractical to try all possible input strings. Thus, we
	// randomly sample random strings from the domain. Thus, perform numSamples
	// trials per size class.
	const numSamples = 128

	buf := bytes.NewBuffer(nil)
	bb := bits.NewBuffer(nil)
	mw := new(Writer)

	// Print titles of each category to compute metrics on:
	//	NumBytes: The length of the input string.
	//	FullDomain: Are all possible strings of this length encodable?
	//	Coverage: What percentage of all possible strings of this length can be encoded?
	//	Efficiency: The efficiency of encoding; this compares NumBytes to EncSize.
	//	EncSize: The size of the output when encoding a string of this length.
	t.Log("NumBytes  FullDomain  Coverage   Efficiency[max=>avg=>min]  EncSize[min<=avg<=max]")

	rand := rand.New(rand.NewSource(0))
	for numBytes := MinRawBytes; numBytes <= MaxRawBytes; numBytes++ {
		numBits := numBytes * 8

		// Whether a string is encodable or not is entirely based on the number
		// of one bits and zero bits in the string. Thus, we gather results from
		// every possible size class.
		encodable := big.NewInt(0) // Total number of input strings that are encodable
		encMin, encMax, encTotal := math.MaxInt8, math.MinInt8, 0.0
		for zeros := 0; zeros <= numBits; zeros++ {
			ones := numBits - zeros

			// If we get a non-zero huffLen, then a string with this many zero
			// bits and one bits is ensured to be encodable.
			if huffLen := mw.computeHuffLen(zeros, ones); huffLen == 0 {
				continue
			}

			// The total number of unique strings with the given number of zero
			// bits and one bits is the combination nCk where n is the total
			// number of bits and k is the the total number of one bits.
			num := comb(int64(numBits), int64(ones))
			encodable.Add(encodable, num)

			// For numSamples trials, keep track of the minimum, average, and
			// maximum size of the encoded output.
			encAvg := 0.0
			for i := 0; i < numSamples; i++ {
				// Generate a random string permutation.
				bb.Reset()
				for _, x := range rand.Perm(numBits) {
					bb.WriteBit(x >= zeros)
				}

				// Encode the string and compute the output length.
				buf.Reset()
				cnt, err := mw.encodeBlock(buf, bb.Bytes(), LastNil)
				assert.Nil(t, err)
				encMin = min(encMin, cnt)
				encMax = max(encMax, cnt)
				encAvg += float64(cnt) / float64(numSamples)
			}

			// Weighted total based on the number of strings.
			encTotal += itof(num) * encAvg
		}

		// If no input string is encodable, don't bother printing results.
		if encodable.Cmp(new(big.Int)) == 0 {
			continue
		}

		encAvg := encTotal / itof(encodable)                              // encAvg     := encTotal / encodable
		domain := new(big.Int).Lsh(big.NewInt(1), uint(numBits))          // domain     := 1 << numBits
		fullDomain := encodable.Cmp(domain) == 0                          // fullDomain := encodable == domain
		coverage := 100.0 * rtof(new(big.Rat).SetFrac(encodable, domain)) // coverage   := 100.0 * (encodable / domain)
		maxEff := 100.0 * (float64(numBytes) / float64(encMin))           // maxEff     := 100.0 *  (numBytes / encMin)
		avgEff := 100.0 * (float64(numBytes) / float64(encAvg))           // avgEff     := 100.0 *  (numBytes / encAvg)
		minEff := 100.0 * (float64(numBytes) / float64(encMax))           // minEff     := 100.0 *  (numBytes / encMax)

		t.Logf("%8d%12v%9.2f%%  [%5.1f%% => %4.1f%% => %4.1f%%]     [%2d <= %4.2f <= %2d]\n",
			numBytes, fullDomain, coverage, maxEff, avgEff, minEff, encMin, encAvg, encMax)
	}
}
