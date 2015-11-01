// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"
import "io/ioutil"

type Reader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read

	rd     bitReader // Input source
	step   func()    // Single step of decompression work (can panic)
	toRead []byte    // Uncompressed data ready to be emitted from Read
	blkLen int       // Uncompressed bytes left to read in meta-block
	insLen int       // Bytes left to insert in current command
	cpyLen int       // Bytes left to copy in current command
	last   bool      // Last block bit detected
	err    error     // Persistent error

	dict    dictDecoder  // Dynamic sliding dictionary
	iacBlk  blockDecoder // Insert-and-copy block decoder
	litBlk  blockDecoder // Literal block decoder
	distBlk blockDecoder // Distance block decoder

	// Literal decoding state fields.
	litMapType []uint8 // The current literal context map for the current block type
	litMap     []uint8 // Literal context map
	cmode      uint8   // The current context mode
	cmodes     []uint8 // Literal context modes

	// Distance decoding state fields.
	distMap     []uint8 // Distance context map
	distMapType []uint8 // The current distance context map for the current block type
	dist        int     // The current distance (may not be in dists)
	dists       [4]int  // Last few distances (newest-to-oldest)
	distZero    bool    // Implicit zero distance symbol found
	npostfix    uint8   // Postfix bits used in distance decoding
	ndirect     uint8   // Number of direct distance codes

	// Static dictionary state fields.
	word    []byte            // Transformed word obtained from static dictionary
	wordBuf [maxWordSize]byte // Buffer to write a transformed word into

	metaWr  io.Writer // Writer to write meta data to
	metaBuf []byte    // Scratch space for reading meta data
}

type blockDecoder struct {
	numTypes int             // Total number of types
	typeLen  int             // The number of blocks left for this type
	types    [2]uint8        // The current (0) and previous (1) block type
	decType  prefixDecoder   // Prefix decoder for the type symbol
	decLen   prefixDecoder   // Prefix decoder for block length
	prefixes []prefixDecoder // Prefix decoders for each block type
}

func NewReader(r io.Reader) *Reader {
	br := new(Reader)
	br.Reset(r)
	return br
}

func (br *Reader) Read(buf []byte) (int, error) {
	for {
		if len(br.toRead) > 0 {
			cnt := copy(buf, br.toRead)
			br.toRead = br.toRead[cnt:]
			br.OutputOffset += int64(cnt)
			return cnt, nil
		}
		if br.err != nil {
			return 0, br.err
		}

		// Perform next step in decompression process.
		func() {
			defer errRecover(&br.err)
			br.step()
		}()
		br.InputOffset = br.rd.offset
		if br.err != nil {
			br.toRead = br.dict.ReadFlush() // Flush what's left in case of error
		}
	}
}

func (br *Reader) Close() error {
	if br.err == io.EOF || br.err == io.ErrClosedPipe {
		br.toRead = nil // Make sure future reads fail
		br.err = io.ErrClosedPipe
		return nil
	}
	return br.err // Return the persistent error
}

func (br *Reader) Reset(r io.Reader) error {
	*br = Reader{
		rd:   br.rd,
		step: br.readStreamHeader,

		dict:    br.dict,
		iacBlk:  br.iacBlk,
		litBlk:  br.litBlk,
		distBlk: br.distBlk,
		word:    br.word[:0],
		cmodes:  br.cmodes[:0],
		litMap:  br.litMap[:0],
		distMap: br.distMap[:0],
		dists:   [4]int{4, 11, 15, 16}, // RFC section 4

		// TODO(dsnet): Should we write meta data somewhere useful?
		metaWr:  ioutil.Discard,
		metaBuf: br.metaBuf,
	}
	br.rd.Init(r)
	return nil
}

// readStreamHeader reads the Brotli stream header according to RFC section 9.1.
func (br *Reader) readStreamHeader() {
	wbits := uint(br.rd.ReadSymbol(&decWinBits))
	if wbits == 0 {
		panic(ErrCorrupt) // Reserved value used
	}
	size := int(1<<wbits) - 16
	br.dict.Init(size)
	br.readBlockHeader()
}

// readBlockHeader reads a meta-block header according to RFC section 9.2.
func (br *Reader) readBlockHeader() {
	if br.last {
		if br.rd.ReadPads() > 0 {
			panic(ErrCorrupt)
		}
		panic(io.EOF)
	}

	// Read ISLAST and ISLASTEMPTY.
	if br.last = br.rd.ReadBits(1) == 1; br.last {
		if empty := br.rd.ReadBits(1) == 1; empty {
			br.readBlockHeader() // Next call will terminate stream
			return
		}
	}

	// Read MLEN and MNIBBLES and process meta data.
	var blkLen int // 1..1<<24
	if nibbles := br.rd.ReadBits(2) + 4; nibbles == 7 {
		if reserved := br.rd.ReadBits(1) == 1; reserved {
			panic(ErrCorrupt)
		}

		var skipLen int // 0..1<<24
		if skipBytes := br.rd.ReadBits(2); skipBytes > 0 {
			skipLen = int(br.rd.ReadBits(skipBytes * 8))
			if skipBytes > 1 && skipLen>>((skipBytes-1)*8) == 0 {
				panic(ErrCorrupt) // Shortest representation not used
			}
			skipLen++
		}

		if br.rd.ReadPads() > 0 {
			panic(ErrCorrupt)
		}
		br.blkLen = skipLen // Use blkLen to track meta data number of bytes
		br.readMetaData()
		return
	} else {
		blkLen = int(br.rd.ReadBits(nibbles * 4))
		if nibbles > 4 && blkLen>>((nibbles-1)*4) == 0 {
			panic(ErrCorrupt) // Shortest representation not used
		}
		blkLen++
	}
	br.blkLen = blkLen

	// Read ISUNCOMPRESSED and process uncompressed data.
	if !br.last {
		if uncompressed := br.rd.ReadBits(1) == 1; uncompressed {
			if br.rd.ReadPads() > 0 {
				panic(ErrCorrupt)
			}
			br.readRawData()
			return
		}
	}
	br.readPrefixCodes()
}

// readMetaData reads meta data according to RFC section 9.2.
func (br *Reader) readMetaData() {
	rd := io.LimitReader(&br.rd, int64(br.blkLen))
	br.metaBuf = extendUint8s(br.metaBuf, 4096) // Lazy allocate
	if cnt, err := io.CopyBuffer(br.metaWr, rd, br.metaBuf); err != nil {
		panic(err)
	} else if cnt < int64(br.blkLen) {
		panic(io.ErrUnexpectedEOF)
	}
	br.readBlockHeader()
}

// readRawData reads raw data according to RFC section 9.2.
func (br *Reader) readRawData() {
	buf := br.dict.WriteSlice()
	if len(buf) > br.blkLen {
		buf = buf[:br.blkLen]
	}

	cnt, err := br.rd.Read(buf)
	br.blkLen -= cnt
	br.dict.WriteMark(cnt)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		panic(err)
	}

	if br.blkLen > 0 {
		br.toRead = br.dict.ReadFlush()
		br.step = br.readRawData // We need to continue this work
		return
	}
	br.readBlockHeader()
}

// readPrefixCodes reads the prefix codes according to RFC section 9.2.
func (br *Reader) readPrefixCodes() {
	// Read block types for literal, insert-and-copy, and distance blocks.
	for _, bd := range []*blockDecoder{&br.litBlk, &br.iacBlk, &br.distBlk} {
		// Note: According to RFC section 6, it is okay for the block count to
		// *not* count down to zero. Thus, there is no need to validate that
		// typeLen is within some reasonable range.
		bd.types = [2]uint8{0, 1}
		bd.typeLen = -1 // Stay on this type until next meta-block

		bd.numTypes = int(br.rd.ReadSymbol(&decCounts)) // 1..256
		if bd.numTypes >= 2 {
			br.rd.ReadPrefixCode(&bd.decType, uint(bd.numTypes)+2)
			br.rd.ReadPrefixCode(&bd.decLen, uint(numBlkCntSyms))
			sym := br.rd.ReadSymbol(&bd.decLen)
			bd.typeLen = int(br.rd.ReadOffset(sym, blkLenRanges))
		}
	}

	// Read NPOSTFIX and NDIRECT.
	npostfix := br.rd.ReadBits(2)            // 0..3
	ndirect := br.rd.ReadBits(4) << npostfix // 0..120
	br.npostfix, br.ndirect = uint8(npostfix), uint8(ndirect)
	numDistSyms := 16 + ndirect + 48<<npostfix

	// Read CMODE, the literal context modes.
	br.cmodes = extendUint8s(br.cmodes, br.litBlk.numTypes)
	for i := range br.cmodes {
		br.cmodes[i] = uint8(br.rd.ReadBits(2))
	}
	br.cmode = br.cmodes[0] // 0..3

	// Read CMAPL, the literal context map.
	numLitTrees := int(br.rd.ReadSymbol(&decCounts)) // 1..256
	br.litMap = extendUint8s(br.litMap, maxLitContextIDs*br.litBlk.numTypes)
	if numLitTrees >= 2 {
		br.rd.ReadContextMap(br.litMap, uint(numLitTrees))
	} else {
		for i := range br.litMap {
			br.litMap[i] = 0
		}
	}
	br.litMapType = br.litMap[0:] // First block type is zero

	// Read CMAPD, the distance context map.
	numDistTrees := int(br.rd.ReadSymbol(&decCounts)) // 1..256
	br.distMap = extendUint8s(br.distMap, maxDistContextIDs*br.distBlk.numTypes)
	if numDistTrees >= 2 {
		br.rd.ReadContextMap(br.distMap, uint(numDistTrees))
	} else {
		for i := range br.distMap {
			br.distMap[i] = 0
		}
	}
	br.distMapType = br.distMap[0:] // First block type is zero

	// Read HTREEL[], HTREEI[], and HTREED[], the arrays of prefix codes.
	br.litBlk.prefixes = extendDecoders(br.litBlk.prefixes, numLitTrees)
	for i := range br.litBlk.prefixes {
		br.rd.ReadPrefixCode(&br.litBlk.prefixes[i], numLitSyms)
	}
	br.iacBlk.prefixes = extendDecoders(br.iacBlk.prefixes, br.iacBlk.numTypes)
	for i := range br.iacBlk.prefixes {
		br.rd.ReadPrefixCode(&br.iacBlk.prefixes[i], numInsSyms)
	}
	br.distBlk.prefixes = extendDecoders(br.distBlk.prefixes, numDistTrees)
	for i := range br.distBlk.prefixes {
		br.rd.ReadPrefixCode(&br.distBlk.prefixes[i], numDistSyms)
	}

	br.readCommand()
}

// readCommand reads start of block command according to RFC section 9.3.
func (br *Reader) readCommand() {
	if br.blkLen == 0 {
		br.readBlockHeader() // Block is complete, read next one
		return
	}

	if br.iacBlk.typeLen == 0 {
		br.iacBlk.readBlockSwitch(&br.rd)
	}
	br.iacBlk.typeLen--

	iacSym := br.rd.ReadSymbol(&br.iacBlk.prefixes[br.iacBlk.types[0]])
	insSym, cpySym := br.decodeInsertAndCopySymbol(iacSym)
	br.insLen = int(br.rd.ReadOffset(insSym, insLenRanges)) // 0..16799809
	br.cpyLen = int(br.rd.ReadOffset(cpySym, cpyLenRanges)) // 2..16779333
	br.distZero = iacSym < 128

	if br.insLen > 0 {
		br.readLiterals()
	} else {
		br.readDistance()
	}
}

// readLiterals reads insLen literal symbols as uncompressed data according to
// RFC section 9.3.
func (br *Reader) readLiterals() {
	buf := br.dict.WriteSlice()
	if len(buf) > br.insLen {
		buf = buf[:br.insLen]
	}

	p1, p2 := br.dict.LastBytes()
	for i := range buf {
		if br.litBlk.typeLen == 0 {
			br.litBlk.readBlockSwitch(&br.rd)
			br.litMapType = br.litMap[64*int(br.litBlk.types[0]):]
			br.cmode = br.cmodes[br.litBlk.types[0]] // 0..3
		}
		br.litBlk.typeLen--

		cidl := getLitContextID(p1, p2, br.cmode) // 0..63
		treel := &br.litBlk.prefixes[br.litMapType[cidl]]
		litSym := br.rd.ReadSymbol(treel)

		buf[i] = byte(litSym)
		p1, p2 = byte(litSym), p1
		br.dict.WriteMark(1)
	}
	br.insLen -= len(buf)
	br.blkLen -= len(buf)

	if br.insLen > 0 {
		br.toRead = br.dict.ReadFlush()
		br.step = br.readLiterals // We need to continue this work
		return
	} else if br.blkLen < 0 {
		panic(ErrCorrupt)
	} else if br.blkLen == 0 {
		br.readCommand()
	} else {
		br.readDistance()
	}
}

// readDistance reads the distance length and copies a sub-string from either
// the past history or from the static dictionary according to RFC section 9.3.
func (br *Reader) readDistance() {
	if br.distZero {
		br.dist = br.dists[0]
	} else {
		if br.distBlk.typeLen == 0 {
			br.distBlk.readBlockSwitch(&br.rd)
			br.distMapType = br.distMap[4*int(br.distBlk.types[0]):]
		}
		br.distBlk.typeLen--

		cidd := getDistContextID(br.cpyLen) // 0..3
		treed := &br.distBlk.prefixes[br.distMapType[cidd]]
		distSym := br.rd.ReadSymbol(treed)

		br.dist = br.decodeDistanceSymbol(distSym)
		br.distZero = bool(distSym == 0)
	}

	if br.dist <= 0 {
		panic(ErrCorrupt)
	}
	if br.dist <= br.dict.HistSize() {
		if !br.distZero {
			copy(br.dists[1:], br.dists[:])
			br.dists[0] = br.dist
		}
		br.copyDynamicDict()
	} else {
		br.copyStaticDict()
	}
}

// copyDynamicDict copies a sub-string from the past according to RFC section 2.
func (br *Reader) copyDynamicDict() {
	cnt := br.dict.WriteCopy(br.dist, br.cpyLen)
	br.blkLen -= cnt
	br.cpyLen -= cnt

	if br.cpyLen > 0 {
		br.toRead = br.dict.ReadFlush()
		br.step = br.copyDynamicDict // We need to continue this work
		return
	}
	if br.blkLen < 0 {
		panic(ErrCorrupt)
	}
	br.readCommand()
}

// copyStaticDict copies a string a string from the static dictionary using
// the logic described in RFC section 8.
func (br *Reader) copyStaticDict() {
	if len(br.word) == 0 {
		if br.cpyLen < minDictLen || br.cpyLen > maxDictLen {
			panic(ErrCorrupt)
		}
		wordIdx := br.dist - (br.dict.HistSize() + 1)
		index := wordIdx % dictSizes[br.cpyLen]
		offset := dictOffsets[br.cpyLen] + index*br.cpyLen
		baseWord := dictLUT[offset : offset+br.cpyLen]
		transformIdx := wordIdx >> uint(dictBitSizes[br.cpyLen])
		if transformIdx >= len(transformLUT) {
			panic(ErrCorrupt)
		}
		cnt := transformWord(br.wordBuf[:], baseWord, transformIdx)
		br.word = br.wordBuf[:cnt]
	}

	buf := br.dict.WriteSlice()
	cnt := copy(buf, br.word)
	br.word = br.word[cnt:]
	br.blkLen -= cnt
	br.dict.WriteMark(cnt)

	if len(br.word) > 0 {
		br.toRead = br.dict.ReadFlush()
		br.step = br.copyStaticDict // We need to continue this work
		return
	}
	if br.blkLen < 0 {
		panic(ErrCorrupt)
	}
	br.readCommand()
}

// decodeInsertAndCopySymbol converts an insert-and-copy length symbol to a pair
// of insert length and copy length symbols according to RFC section 5.
func (br *Reader) decodeInsertAndCopySymbol(iacSym uint) (insSym, cpySym uint) {
	// TODO(dsnet): Results for symbols 0..703 can be determined by a LUT.
	switch iacSym / 64 {
	case 0, 2: // 0..63 and 128..191
		insSym, cpySym = 0, 0
	case 1, 3: // 64..127 and 192..255
		insSym, cpySym = 0, 8
	case 4: // 256..319
		insSym, cpySym = 8, 0
	case 5: // 320..383
		insSym, cpySym = 8, 8
	case 6: // 384..447
		insSym, cpySym = 0, 16
	case 7: // 448..511
		insSym, cpySym = 16, 0
	case 8: // 512..575
		insSym, cpySym = 8, 16
	case 9: // 576..639
		insSym, cpySym = 16, 8
	case 10: // 640..703
		insSym, cpySym = 16, 16
	}

	r64 := iacSym % 64
	insSym += r64 >> 3   // Lower 3 bits
	cpySym += r64 & 0x07 // Upper 3 bits
	return insSym, cpySym
}

// decodeDistanceSymbol decodes distSym returns the effective backward distance
// according to RFC section 4.
func (br *Reader) decodeDistanceSymbol(distSym uint) (dist int) {
	// TODO(dsnet): Results for symbols 0..15 can be determined by a LUT.
	switch {
	case distSym < 4: // Last to fourth-to-last distance
		return br.dists[distSym]
	case distSym < 10: // Variations on last distance
		delta := int(distSym/2 - 1) // 1..3
		if distSym%2 == 0 {
			delta *= -1
		}
		return br.dists[0] + delta
	case distSym < 16: // Variations on second-to-last distance
		delta := int(distSym/2 - 4) // 1..3
		if distSym%2 == 0 {
			delta *= -1
		}
		return br.dists[1] + delta
	case distSym < uint(16+br.ndirect): // Direct distance
		return int(distSym - 15) // 1..ndirect
	default:
		distSym -= uint(16 + br.ndirect)
		postfixMask := uint(1<<br.npostfix - 1)
		hcode := distSym >> br.npostfix
		lcode := distSym & postfixMask
		nbits := 1 + distSym>>(br.npostfix+1)
		offset := ((2 + (hcode & 1)) << nbits) - 4
		dextra := br.rd.ReadBits(nbits)
		return int(((offset + dextra) << br.npostfix) + lcode + uint(br.ndirect) + 1)
	}
}

// readBlockSwitch handles a block switch command according to RFC section 6.
func (bd *blockDecoder) readBlockSwitch(r *bitReader) {
	symType := r.ReadSymbol(&bd.decType)
	switch symType {
	case 0:
		symType = uint(bd.types[1])
	case 1:
		symType = uint(bd.types[0]) + 1
		if symType >= uint(bd.numTypes) {
			symType -= uint(bd.numTypes)
		}
	default:
		symType -= 2
	}
	bd.types = [2]uint8{uint8(symType), bd.types[0]}

	symLen := r.ReadSymbol(&bd.decLen)
	bd.typeLen = int(r.ReadOffset(symLen, blkLenRanges))
}

func extendUint8s(s []uint8, n int) []uint8 {
	if cap(s) >= n {
		return s[:n]
	}
	return append(s[:cap(s)], make([]uint8, n-cap(s))...)
}

func extendDecoders(s []prefixDecoder, n int) []prefixDecoder {
	if cap(s) >= n {
		return s[:n]
	}
	return append(s[:cap(s)], make([]prefixDecoder, n-cap(s))...)
}
