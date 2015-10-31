// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"
import "bufio"

// TODO(dsnet): Most of this logic is identical to compress/flate.
// Centralize common logic to compress/internal/prefix.

type byteReader interface {
	io.Reader
	io.ByteReader
}

type bitReader struct {
	rd      byteReader
	prefix  prefixDecoder // Local prefix decoder
	bufBits uint32        // Buffer to hold some bits
	numBits uint          // Number of valid bits in bufBits
	offset  int64         // Number of bytes read from the underlying io.Reader
}

func (br *bitReader) Init(r io.Reader) {
	*br = bitReader{prefix: br.prefix}
	if rr, ok := r.(byteReader); ok {
		br.rd = rr
	} else {
		br.rd = bufio.NewReader(r)
	}
}

// Read reads up to len(buf) bytes into buf.
func (br *bitReader) Read(buf []byte) (int, error) {
	if br.numBits > 0 {
		return 0, Error("non-empty bit buffer")
	}
	cnt, err := br.rd.Read(buf)
	br.offset += int64(cnt)
	return cnt, err
}

// ReadBits reads nb bits in LSB order from the underlying reader.
// If an IO error occurs, then it panics.
func (br *bitReader) ReadBits(nb uint) uint {
	for br.numBits < nb {
		c, err := br.rd.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			panic(err)
		}
		br.offset++
		br.bufBits |= uint32(c) << br.numBits
		br.numBits += 8
	}
	val := uint(br.bufBits & uint32(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}

// ReadPads reads 0-7 bits from the bit buffer to achieve byte-alignment.
func (br *bitReader) ReadPads() uint {
	nb := br.numBits % 8
	val := uint(br.bufBits & uint32(1<<nb-1))
	br.bufBits >>= nb
	br.numBits -= nb
	return val
}

// ReadSymbol reads the next prefix symbol using the provided prefixDecoder.
// If an IO error occurs, then it panics.
func (br *bitReader) ReadSymbol(pd *prefixDecoder) uint {
	if len(pd.chunks) == 0 {
		panic(ErrCorrupt) // Decode with empty tree
	}

	nb := uint(pd.minBits)
	for {
		for br.numBits < nb {
			// This section is an inlined version of the inner loop of ReadBits
			// for performance reasons.
			c, err := br.rd.ReadByte()
			if err != nil {
				if err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				panic(err)
			}
			br.offset++
			br.bufBits |= uint32(c) << br.numBits
			br.numBits += 8
		}
		chunk := pd.chunks[uint16(br.bufBits)&pd.chunkMask]
		nb = uint(chunk & prefixCountMask)
		if nb > uint(pd.chunkBits) {
			linkIdx := chunk >> prefixCountBits
			chunk = pd.links[linkIdx][uint16(br.bufBits>>pd.chunkBits)&pd.linkMask]
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

// ReadPrefixCode reads the prefix definition from the stream and initializes
// the provided prefixDecoder. The value maxSyms is the alphabet size of the
// prefix code being generated. The actual number of representable symbols
// will be between 1 and maxSyms, inclusively.
func (br *bitReader) ReadPrefixCode(pd *prefixDecoder, maxSyms uint) {
	hskip := br.ReadBits(2)
	if hskip == 1 {
		br.readSimplePrefixCode(pd, maxSyms)
	} else {
		br.readComplexPrefixCode(pd, maxSyms, hskip)
	}
}

// readSimplePrefixCode reads the prefix code according to RFC section 3.4.
func (br *bitReader) readSimplePrefixCode(pd *prefixDecoder, maxSyms uint) {
	var codes [4]prefixCode
	nsym := int(br.ReadBits(2)) + 1
	clen := neededBits(uint16(maxSyms))
	for i := 0; i < nsym; i++ {
		codes[i].sym = uint16(br.ReadBits(clen))
	}

	var copyLens = func(lens []uint) {
		for i := 0; i < nsym; i++ {
			codes[i].len = uint8(lens[i])
		}
	}
	var compareSwap = func(i, j int) {
		if codes[i].sym > codes[j].sym {
			codes[i], codes[j] = codes[j], codes[i]
		}
	}

	switch nsym {
	case 1:
		copyLens(simpleLens1[:])
	case 2:
		copyLens(simpleLens2[:])
		compareSwap(0, 1)
	case 3:
		copyLens(simpleLens3[:])
		compareSwap(0, 1)
		compareSwap(0, 2)
		compareSwap(1, 2)
	case 4:
		if tsel := br.ReadBits(1) == 1; !tsel {
			copyLens(simpleLens4a[:])
		} else {
			copyLens(simpleLens4b[:])
		}
		compareSwap(0, 1)
		compareSwap(2, 3)
		compareSwap(0, 2)
		compareSwap(1, 3)
		compareSwap(1, 2)
	}
	if uint(codes[nsym-1].sym) >= maxSyms {
		panic(ErrCorrupt) // Symbol goes beyond range of alphabet
	}
	pd.Init(codes[:nsym], true) // Must have 1..4 symbols
}

// readComplexPrefixCode reads the prefix code according to RFC section 3.5.
func (br *bitReader) readComplexPrefixCode(pd *prefixDecoder, maxSyms, hskip uint) {
	// Read the code-lengths prefix table.
	var codeCLensArr [len(complexLens)]prefixCode // Sorted, but may have holes
	sum := 32
	for _, sym := range complexLens[hskip:] {
		clen := br.ReadSymbol(&decCLens)
		if clen > 0 {
			codeCLensArr[sym] = prefixCode{sym: uint16(sym), len: uint8(clen)}
			if sum -= 32 >> clen; sum <= 0 {
				break
			}
		}
	}
	codeCLens := codeCLensArr[:0] // Compact the array to have no holes
	for _, c := range codeCLensArr {
		if c.len > 0 {
			codeCLens = append(codeCLens, c)
		}
	}
	if len(codeCLens) < 1 {
		panic(ErrCorrupt)
	}
	br.prefix.Init(codeCLens, true) // Must have 1..len(complexLens) symbols

	// Use code-lengths table to decode rest of prefix table.
	var codesArr [maxNumAlphabetSyms]prefixCode
	var sym, repSymLast, repCntLast, clenLast uint = 0, 0, 0, 8
	codes := codesArr[:0]
	for sym, sum = 0, 32768; sym < maxSyms && sum > 0; {
		clen := br.ReadSymbol(&br.prefix)
		if clen < 16 {
			// Literal bit-length symbol used.
			if clen > 0 {
				codes = append(codes, prefixCode{sym: uint16(sym), len: uint8(clen)})
				clenLast = clen
				sum -= 32768 >> clen
			}
			repSymLast = 0 // Reset last repeater symbol
			sym++
		} else {
			// Repeater symbol used.
			//	16: Repeat previous non-zero code-length
			//	17: Repeat code length of zero

			repSym := clen // Rename clen for better clarity
			if repSym != repSymLast {
				repCntLast = 0
				repSymLast = repSym
			}

			nb := repSym - 14          // 2..3 bits
			rep := br.ReadBits(nb) + 3 // 3..6 or 3..10
			if repCntLast > 0 {
				rep += (repCntLast - 2) << nb // Modify previous repeat count
			}
			repDiff := rep - repCntLast // Always positive
			repCntLast = rep

			if repSym == 16 {
				clen := clenLast
				for symEnd := sym + repDiff; sym < symEnd; sym++ {
					codes = append(codes, prefixCode{sym: uint16(sym), len: uint8(clen)})
				}
				sum -= int(repDiff) * (32768 >> clen)
			} else {
				sym += repDiff
			}
		}
	}
	if len(codes) < 2 || sym > maxSyms {
		panic(ErrCorrupt)
	}
	pd.Init(codes, true) // Must have 2..maxSyms symbols
}

// ReadContextMap reads the context map according to RFC section 7.3.
func (br *bitReader) ReadContextMap(cm []uint8, numTrees uint) {
	// TODO(dsnet): Test the following edge cases:
	// * Test with largest and smallest MAXRLE sizes
	// * Test with with very large MAXRLE value
	// * Test inverseMoveToFront

	maxRLE := br.ReadSymbol(&decMaxRLE)
	br.ReadPrefixCode(&br.prefix, maxRLE+numTrees)
	for i := 0; i < len(cm); {
		sym := br.ReadSymbol(&br.prefix)
		if sym == 0 || sym > maxRLE {
			// Single non-zero value.
			if sym > 0 {
				sym -= maxRLE
			}
			cm[i] = uint8(sym)
			i++
		} else {
			// Repeated zeros.
			n := int(br.ReadOffset(sym-1, maxRLERanges))
			if i+n > len(cm) {
				panic(ErrCorrupt)
			}
			for j := i + n; i < j; i++ {
				cm[i] = 0
			}
		}
	}

	if invert := br.ReadBits(1) == 1; invert {
		inverseMoveToFront(cm)
	}
}
