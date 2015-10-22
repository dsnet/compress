// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"

type prefixBlocks struct {
	ntypes   int             // Total number of types
	btype    int             // The current block type
	blen     int             // The count for the current block type
	ptype    prefixDecoder   // Prefix decoder for btype
	plen     prefixDecoder   // Prefix decoder for blen
	prefixes []prefixDecoder // Prefix decoders for each block type
}

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

	dict     dictDecoder  // Dynamic sliding dictionary
	insBlks  prefixBlocks // Insert-and-copy prefix blocks
	litBlks  prefixBlocks // Literal prefix blocks
	distBlks prefixBlocks // Distance prefix blocks
	cmodes   []uint8      // Literal context modes
	litMap   []uint8      // Literal context map
	distMap  []uint8      // Distance context map
	npostfix uint8        // Postfix bits used in distance decoding
	ndirect  uint8        // Number of direct distance codes
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
		step: br.readStreamHeader,
		dict: br.dict,
	}
	br.rd.Init(r)
	return nil
}

// readStreamHeader reads the Brotli stream header according to RFC section 9.1.
func (br *Reader) readStreamHeader() {
	wbits := uint(br.rd.ReadSymbol(&decWinBits))
	if wbits == 0 {
		panic(ErrCorrupt)
	}
	br.dict.Init(wbits)
	br.step = br.readBlockHeader
}

// readBlockHeader reads a meta-block header according to RFC section 9.2.
func (br *Reader) readBlockHeader() {
	if br.last {
		// TODO(dsnet): Flush data?
		if br.rd.ReadPads() > 0 {
			panic(ErrCorrupt)
		}
		br.err = io.EOF
		return
	}

	// Read ISLAST and ISLASTEMPTY.
	if br.last = br.rd.ReadBits(1) == 1; br.last {
		if empty := br.rd.ReadBits(1) == 1; empty {
			br.step = br.readBlockHeader // Next call will terminate stream
			return
		}
	}

	// Read MLEN and MNIBBLES and process meta data.
	var blkLen int // Valid values are [1..1<<24]
	if nibbles := br.rd.ReadBits(2) + 4; nibbles == 7 {
		if reserved := br.rd.ReadBits(1) == 1; reserved {
			panic(ErrCorrupt)
		}

		var skipLen int // Valid values are [0..1<<24]
		if skipBytes := br.rd.ReadBits(2); skipBytes > 0 {
			skipLen = int(br.rd.ReadBits(skipBytes * 8))
			if skipBytes > 1 && skipLen>>((skipBytes-1)*8) == 0 {
				panic(ErrCorrupt) // Shortest representation not used
			}
			skipLen++
		}

		// TODO(dsnet): Should we do something with this meta data?
		// TODO(dsnet): Avoid allocating a large buffer to read data.
		if br.rd.ReadPads() > 0 {
			panic(ErrCorrupt)
		}
		if _, err := io.ReadFull(&br.rd, make([]byte, skipLen)); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			panic(err)
		}
		br.step = br.readBlockHeader
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
			br.step = br.readRawData
			return
		}
	}

	br.readPrefixCodes()
}

// readPrefixCodes reads the prefix codes according to RFC section 9.2.
func (br *Reader) readPrefixCodes() {
	// Read block types for literal, insert-and-copy, and distance blocks.
	for _, pb := range []*prefixBlocks{&br.litBlks, &br.insBlks, &br.distBlks} {
		pb.btype = 0
		pb.blen = 1 << 28 // Large enough value that will stay positive

		pb.ntypes = int(br.rd.ReadSymbol(&decCounts))
		if pb.ntypes >= 2 {
			br.rd.ReadPrefixCode(&pb.ptype, pb.ntypes+2)
			br.rd.ReadPrefixCode(&pb.plen, numCntSyms)
			sym := int(br.rd.ReadSymbol(&pb.plen))
			_ = sym
			// TODO(dsnet): Read BLEN_x
			// TODO(dsnet): Initialize second-to-last and last block types.
		}
	}

	// Read NPOSTFIX and NDIRECT.
	npostfix := br.rd.ReadBits(2)            // Valid values are [0..3]
	ndirect := br.rd.ReadBits(4) << npostfix // Valid values are [0..120]
	br.npostfix, br.ndirect = uint8(npostfix), uint8(ndirect)
	numDistSyms := int(16 + ndirect + (48 << npostfix))

	// Read CMODE, the literal context modes.
	br.cmodes = extendUint8s(br.cmodes, br.litBlks.ntypes)
	for i := range br.cmodes {
		br.cmodes[i] = uint8(br.rd.ReadBits(2))
	}

	// Read CMAPL, the literal context map.
	numLitTrees := int(br.rd.ReadSymbol(&decCounts))
	br.litMap = extendUint8s(br.litMap, maxLitContextIDs*br.litBlks.ntypes)
	if numLitTrees >= 2 {
		br.rd.ReadContextMap(br.litMap, numLitTrees)
	} else {
		for i := range br.litMap {
			br.litMap[i] = 0
		}
	}

	// Read CMAPD, the distance context map.
	numDistTrees := int(br.rd.ReadSymbol(&decCounts))
	br.distMap = extendUint8s(br.distMap, maxDistContextIDs*br.distBlks.ntypes)
	if numDistTrees >= 2 {
		br.rd.ReadContextMap(br.distMap, numDistTrees)
	} else {
		for i := range br.distMap {
			br.distMap[i] = 0
		}
	}

	// Read HTREEL[], HTREEI[], and HTREED[], the arrays of prefix codes.
	br.litBlks.prefixes = extendDecoders(br.litBlks.prefixes, numLitTrees)
	for i := range br.litBlks.prefixes {
		br.rd.ReadPrefixCode(&br.litBlks.prefixes[i], numLitSyms)
	}
	br.insBlks.prefixes = extendDecoders(br.insBlks.prefixes, br.insBlks.ntypes)
	for i := range br.insBlks.prefixes {
		br.rd.ReadPrefixCode(&br.insBlks.prefixes[i], numInsSyms)
	}
	br.distBlks.prefixes = extendDecoders(br.distBlks.prefixes, numDistTrees)
	for i := range br.distBlks.prefixes {
		br.rd.ReadPrefixCode(&br.distBlks.prefixes[i], numDistSyms)
	}

	br.step = br.readBlockData
}

// readRawData reads raw data according to RFC section 9.2.
func (br *Reader) readRawData() {
	if br.blkLen == 0 {
		br.step = br.readBlockHeader
		return
	}

	// TODO(dsnet): Handle sliding windows properly.
	// TODO(dsnet): Avoid allocating a large buffer to read data.
	if len(br.toRead) > 0 {
		return
	}
	buf := make([]byte, br.blkLen)
	cnt, err := br.rd.Read(buf)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		panic(err)
	}
	br.toRead = buf[:cnt]
	br.blkLen -= cnt
	br.step = br.readRawData
}

// readBlockData reads block data according to RFC section 9.3.
func (br *Reader) readBlockData() {
	if br.blkLen == 0 {
		br.step = br.readBlockHeader
		return
	}

	/*
		if BLEN_I is zero
			read block type using HTREE_BTYPE_I and set BTYPE_I
				save previous block type
			read block count using HTREE_BLEN_I and set BLEN_I
		decrement BLEN_I
		read insert and copy length, ILEN, CLEN with HTREEI[BTYPE_I]
		loop for ILEN
			if BLEN_L is zero
				read block type using HTREE_BTYPE_L and set BTYPE_L
					save previous block type
				read block count using HTREE_BLEN_L and set BLEN_L
			decrement BLEN_L
			look up context mode CMODE[BTYPE_L]
			compute context ID, CIDL from last two uncompressed bytes
			read literal using HTREEL[CMAPL[64 * BTYPE_L + CIDL]]
			write literal to uncompressed stream
		if number of uncompressed bytes produced in the loop for
			this meta-block is MLEN, then break from loop (in this
			case the copy length is ignored and can have any value)
		if distance code is implicit zero from insert-and-copy code
			set backward distance to the last distance
		else
			if BLEN_D is zero
				read block type using HTREE_BTYPE_D and set BTYPE_D
					save previous block type
				read block count using HTREE_BLEN_D and set BLEN_D
			decrement BLEN_D
			compute context ID, CIDD from CLEN
			read distance code with HTREED[CMAPD[4 * BTYPE_D + CIDD]]
			compute distance by distance short code substitution
		move backwards distance bytes in the uncompressed data and
			copy CLEN bytes from this position to the uncompressed
			stream, or look up the static dictionary word, transform
			the word as directed, and copy the result to the
			uncompressed stream
	*/
	if br.blkLen < 0 {
		panic(ErrCorrupt)
	}

	br.step = br.readBlockData
}

// extendUint8s returns a slice with length n, reusing s if possible.
func extendUint8s(s []uint8, n int) []uint8 {
	if cap(s) >= n {
		return s[:n]
	}
	return append(s[:cap(s)], make([]uint8, n-cap(s))...)
}

// extendDecoders returns a slice with length n, reusing s if possible.
func extendDecoders(s []prefixDecoder, n int) []prefixDecoder {
	if cap(s) >= n {
		return s[:n]
	}
	return append(s[:cap(s)], make([]prefixDecoder, n-cap(s))...)
}
