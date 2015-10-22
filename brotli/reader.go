// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

import "io"

type Reader struct {
	InputOffset  int64 // Total number of bytes read from underlying io.Reader
	OutputOffset int64 // Total number of bytes emitted from Read

	rd     bitReader // Input source
	step   func()    // Single step of decompression work (can panic)
	blkLen int       // Uncompressed bytes left to read in meta-block
	wsize  int       // Sliding window size
	wdict  []byte    // Sliding window history, dynamically grown to match wsize
	toRead []byte    // Uncompressed data ready to be emitted from Read
	last   bool      // Last block bit detected
	err    error     // Persistent error
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
		return nil
	}
	err := br.err
	br.err = io.ErrClosedPipe
	return err
}

func (br *Reader) Reset(r io.Reader) error {
	*br = Reader{
		step:  br.readStreamHeader,
		wdict: br.wdict[:0],
	}
	br.rd.Init(r)
	return nil
}

// readStreamHeader reads the Brotli stream header according to RFC section 9.1.
func (br *Reader) readStreamHeader() {
	var wbits uint
	if val := br.rd.ReadBits(1); val != 1 { // Code is "0"
		wbits = 16
		goto done
	}
	if val := br.rd.ReadBits(3); val != 0 { // Code is "1xxx"
		wbits = 18 + uint(val-1)
		goto done
	}
	if val := br.rd.ReadBits(3); val != 1 { // Code is "1000xxx"
		if val == 0 {
			val = 9
		}
		wbits = 10 + uint(val-2)
		goto done
	}
	panic(ErrCorrupt) // Code is "1000100", which is invalid

done:
	// Regardless of what wsize claims, start with a small dictionary to avoid
	// denial-of-service attacks with large memory allocation.
	br.wsize = (1 << wbits) - 16
	if br.wdict == nil {
		br.wdict = make([]byte, 0, 1024)
	}
	br.wdict = br.wdict[:0]
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
	/*
		loop for each three block categories (i = L, I, D)
			read NBLTYPESi
			if NBLTYPESi >= 2
				read prefix code for block types, HTREE_BTYPE_i
				read prefix code for block counts, HTREE_BLEN_i
				read block count, BLEN_i
				set block type, BTYPE_i to 0
				initialize second-to-last and last block types to 0 and 1
			else
				set block type, BTYPE_i to 0
				set block count, BLEN_i to 268435456
		read NPOSTFIX and NDIRECT
		read array of literal context modes, CMODE[]
		read NTREESL
		if NTREESL >= 2
			read literal context map, CMAPL[]
		else
			fill CMAPL[] with zeros
		read NTREESD
		if NTREESD >= 2
			read distance context map, CMAPD[]
		else
			fill CMAPD[] with zeros
		read array of prefix codes for literals, HTREEL[]
		read array of prefix codes for insert-and-copy, HTREEI[]
		read array of prefix codes for distances, HTREED[]
	*/

	br.step = br.readBlockData
}

// readRawData reads raw data according to RFC section 9.2.
func (br *Reader) readRawData() {
	if br.blkLen <= 0 {
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

// readBlockData reads block data according to RFC section 9.2.
func (br *Reader) readBlockData() {
	if br.blkLen <= 0 { // TODO(dsnet): Can this be negative?
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

	br.step = br.readBlockData
}
