// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package flate

import "io"
import "bufio"

// The bitReader preserves the property that it will never read more bytes than
// is necessary. However, this feature dramatically hurts performance because
// every byte needs to be obtained through a ReadByte method call.
// Furthermore, the decoding of variable length codes in ReadSymbol, often
// requires multiple passes before it knows the exact bit-length of the code.
//
// Thus, to improve performance, if the underlying byteReader is a bufio.Reader,
// then the bitReader will use the Peek and Discard methods to fill the internal
// bit buffer with as many bits as possible, allowing the TryReadBits and
// TryReadSymbol methods to often succeed on the first try.

type byteReader interface {
	io.Reader
	io.ByteReader
}

type bitReader struct {
	rd      byteReader
	bufBits uint64 // Buffer to hold some bits
	numBits uint   // Number of valid bits in bufBits
	offset  int64  // Number of bytes read from the underlying io.Reader

	// These fields are only used if rd is a bufio.Reader.
	bufRd       *bufio.Reader
	bufPeek     []byte // Buffer for the Peek data
	discardBits int    // Number of bits to discard from bufio.Reader
	fedBits     uint   // Number of bits fed in last call to FeedBits

	// Local copy of decoder to reduce memory allocations.
	prefix prefixDecoder
}

func (br *bitReader) Init(r io.Reader) {
	*br = bitReader{prefix: br.prefix}
	if rr, ok := r.(byteReader); ok {
		br.rd = rr
	} else {
		br.rd = bufio.NewReader(r)
	}
	if brd, ok := br.rd.(*bufio.Reader); ok {
		br.bufRd = brd
	}
}

// FlushOffset updates the read offset of the underlying byteReader.
// If the byteReader is a bufio.Reader, then this calls Discard to update the
// read offset.
func (br *bitReader) FlushOffset() int64 {
	if br.bufRd == nil {
		return br.offset
	}

	// Update the number of total bits to discard.
	br.discardBits += int(br.fedBits - br.numBits)
	br.fedBits = br.numBits

	// Discard some bytes to update read offset.
	nd := (br.discardBits + 7) / 8 // Round up to nearest byte
	nd, _ = br.bufRd.Discard(nd)
	br.discardBits -= nd * 8 // -7..0
	br.offset += int64(nd)

	// These are invalid after Discard.
	br.bufPeek = nil
	return br.offset
}

// FeedBits ensures that at least nb bits exist in the bit buffer.
// If the underlying byteReader is a bufio.Reader, then this will fill the
// bit buffer with as many bits as possible, relying on Peek and Discard to
// properly advance the read offset. Otherwise, it will use ReadByte to fill the
// buffer with just the right number of bits.
func (br *bitReader) FeedBits(nb uint) {
	if br.bufRd != nil {
		br.discardBits += int(br.fedBits - br.numBits)
		for {
			if len(br.bufPeek) == 0 {
				br.fedBits = br.numBits // Don't discard bits just added
				br.FlushOffset()

				var err error
				cntPeek := 8 // Minimum Peek amount to make progress
				if br.bufRd.Buffered() > cntPeek {
					cntPeek = br.bufRd.Buffered()
				}
				br.bufPeek, err = br.bufRd.Peek(cntPeek)
				br.bufPeek = br.bufPeek[int(br.numBits/8):] // Skip buffered bits
				if len(br.bufPeek) == 0 {
					if br.numBits >= nb {
						break
					}
					if err == io.EOF {
						err = io.ErrUnexpectedEOF
					}
					panic(err)
				}
			}
			cnt := int(64-br.numBits) / 8
			if cnt > len(br.bufPeek) {
				cnt = len(br.bufPeek)
			}
			for _, c := range br.bufPeek[:cnt] {
				br.bufBits |= uint64(c) << br.numBits
				br.numBits += 8
			}
			br.bufPeek = br.bufPeek[cnt:]
			if br.numBits > 56 {
				break
			}
		}
		br.fedBits = br.numBits
	} else {
		for br.numBits < nb {
			c, err := br.rd.ReadByte()
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				panic(err)
			}
			br.bufBits |= uint64(c) << br.numBits
			br.numBits += 8
			br.offset++
		}
	}
}

// Read reads up to len(buf) bytes into buf.
func (br *bitReader) Read(buf []byte) (cnt int, err error) {
	if br.numBits%8 != 0 {
		return 0, Error("non-aligned bit buffer")
	}
	if br.numBits > 0 {
		for cnt = 0; len(buf) > cnt && br.numBits > 0; cnt++ {
			buf[cnt] = byte(br.bufBits)
			br.bufBits >>= 8
			br.numBits -= 8
		}
	} else {
		br.FlushOffset()
		cnt, err = br.rd.Read(buf)
		br.offset += int64(cnt)
	}
	return cnt, err
}

// TryReadBits attempts to read nb bits using the contents of the bit buffer
// alone. It returns the value and whether it succeeded.
//
// This method is designed to be inlined for performance reasons.
func (br *bitReader) TryReadBits(nb uint) (uint, bool) {
	if br.numBits < nb {
		return 0, false
	}
	val := uint(br.bufBits & uint64(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val, true
}

// ReadBits reads nb bits in LSB order from the underlying reader.
func (br *bitReader) ReadBits(nb uint) uint {
	br.FeedBits(nb)
	val := uint(br.bufBits & uint64(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}

// ReadPads reads 0-7 bits from the bit buffer to achieve byte-alignment.
func (br *bitReader) ReadPads() uint {
	nb := br.numBits % 8
	val := uint(br.bufBits & uint64(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}

// TryReadSymbol attempts to decode the next symbol using the contents of the
// bit buffer alone. It returns the decoded symbol and whether it succeeded.
//
// This method is designed to be inlined for performance reasons.
func (br *bitReader) TryReadSymbol(pd *prefixDecoder) (uint, bool) {
	if br.numBits < uint(pd.minBits) || len(pd.chunks) == 0 {
		return 0, false
	}
	chunk := pd.chunks[uint32(br.bufBits)&pd.chunkMask]
	nb := uint(chunk & prefixCountMask)
	if nb > br.numBits || nb > uint(pd.chunkBits) {
		return 0, false
	}
	br.bufBits >>= nb
	br.numBits -= nb
	return uint(chunk >> prefixCountBits), true
}

// ReadSymbol reads the next prefix symbol using the provided prefixDecoder.
func (br *bitReader) ReadSymbol(pd *prefixDecoder) uint {
	if len(pd.chunks) == 0 {
		panic(ErrCorrupt) // Decode with empty tree
	}

	nb := uint(pd.minBits)
	for {
		br.FeedBits(nb)
		chunk := pd.chunks[uint32(br.bufBits)&pd.chunkMask]
		nb = uint(chunk & prefixCountMask)
		if nb > uint(pd.chunkBits) {
			linkIdx := chunk >> prefixCountBits
			chunk = pd.links[linkIdx][uint32(br.bufBits>>pd.chunkBits)&pd.linkMask]
			nb = uint(chunk & prefixCountMask)
		}
		if nb <= br.numBits {
			br.bufBits >>= nb
			br.numBits -= nb
			return uint(chunk >> prefixCountBits)
		}
	}
}

// ReadOffset reads an offset value using the provided rangesCodes indexed by
// the given symbol.
func (br *bitReader) ReadOffset(sym uint, rcs []rangeCode) uint {
	rc := rcs[sym]
	return uint(rc.base) + br.ReadBits(uint(rc.bits))
}

// ReadPrefixCodes reads the literal and distance prefix codes according to
// RFC section 3.2.7.
func (br *bitReader) ReadPrefixCodes(hl, hd *prefixDecoder) {
	numLitSyms := br.ReadBits(5) + 257
	numDistSyms := br.ReadBits(5) + 1
	numCLenSyms := br.ReadBits(4) + 4
	if numLitSyms > maxNumLitSyms || numDistSyms > maxNumDistSyms {
		panic(ErrCorrupt)
	}

	// Read the code-lengths prefix table.
	var codeCLensArr [maxNumCLenSyms]prefixCode // Sorted, but may have holes
	for _, sym := range clenLens[:numCLenSyms] {
		clen := br.ReadBits(3)
		if clen > 0 {
			codeCLensArr[sym] = prefixCode{sym: uint32(sym), len: uint32(clen)}
		}
	}
	codeCLens := codeCLensArr[:0] // Compact the array to have no holes
	for _, c := range codeCLensArr {
		if c.len > 0 {
			codeCLens = append(codeCLens, c)
		}
	}
	codeCLens = handleDegenerateCodes(codeCLens, maxNumCLenSyms)
	br.prefix.Init(codeCLens, true)

	// Use code-lengths table to decode HLIT and HDIST prefix tables.
	var codesArr [maxNumLitSyms + maxNumDistSyms]prefixCode
	var clenLast uint
	codeLits := codesArr[:0]
	codeDists := codesArr[maxNumLitSyms:maxNumLitSyms]
	appendCode := func(sym, clen uint) {
		if sym < numLitSyms {
			pc := prefixCode{sym: uint32(sym), len: uint32(clen)}
			codeLits = append(codeLits, pc)
		} else {
			pc := prefixCode{sym: uint32(sym - numLitSyms), len: uint32(clen)}
			codeDists = append(codeDists, pc)
		}
	}
	for sym, maxSyms := uint(0), numLitSyms+numDistSyms; sym < maxSyms; {
		clen := br.ReadSymbol(&br.prefix)
		if clen < 16 {
			// Literal bit-length symbol used.
			if clen > 0 {
				appendCode(sym, clen)
			}
			clenLast = clen
			sym++
		} else {
			// Repeater symbol used.
			var repCnt uint
			switch repSym := clen; repSym {
			case 16:
				if sym == 0 {
					panic(ErrCorrupt)
				}
				clen = clenLast
				repCnt = 3 + br.ReadBits(2)
			case 17:
				clen = 0
				repCnt = 3 + br.ReadBits(3)
			case 18:
				clen = 0
				repCnt = 11 + br.ReadBits(7)
			default:
				panic(ErrCorrupt)
			}

			if clen > 0 {
				for symEnd := sym + repCnt; sym < symEnd; sym++ {
					appendCode(sym, clen)
				}
			} else {
				sym += repCnt
			}
			if sym > maxSyms {
				panic(ErrCorrupt)
			}
		}
	}

	codeLits = handleDegenerateCodes(codeLits, maxNumLitSyms)
	hl.Init(codeLits, true)
	codeDists = handleDegenerateCodes(codeDists, maxNumDistSyms)
	hd.Init(codeDists, true)

	// As an optimization, we can initialize minBits to read at a time for the
	// HLIT tree to the length of the EOB marker since we know that every block
	// must terminate with one. This preserves the property that we never read
	// any extra bytes after the end of the DEFLATE stream.
	//
	// This optimization is not helpful if the underlying reader is bufio.Reader
	// since FeedBits always attempts to fill the bit buffer.
	if br.bufRd == nil {
		for i := len(codeLits) - 1; i >= 0; i-- {
			if codeLits[i].sym == 256 && codeLits[i].len > 0 {
				hl.minBits = codeLits[i].len
				break
			}
		}
	}
}

// RFC section 3.2.7 allows degenerate prefix trees with only node, but requires
// a single bit for that node. This causes an unbalanced tree where the "1" code
// is unused. The canonical prefix code generation algorithm breaks with this.
//
// To handle this case, we artificially insert another node for the "1" code
// that uses a symbol larger than the alphabet to force an error later if
// the code ends up getting used.
func handleDegenerateCodes(codes []prefixCode, maxSyms uint) []prefixCode {
	if len(codes) != 1 {
		return codes
	}
	return append(codes, prefixCode{sym: uint32(maxSyms), len: 1})
}
