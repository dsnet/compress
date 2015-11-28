// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

// moveToFront implements both the MTF and RLE stages of bzip2 at the same time.
// Any runs of zeros in the encoded output will be replaced by a single zero,
// and a count will be added to the runs slice.
//
// For example, if the normal MTF output was:
//	idxs: []uint8{0, 0, 1, 6, 3, 0, 0, 0, 2, 1, 0, 4}
//
// Then the actual output will be:
//	idxs: []uint8{0, 1, 6, 3, 0, 2, 1, 0, 4}
//	runs: []uint32{2, 3, 1}
type moveToFront struct {
	dictBuf [256]uint8
	dictLen int

	// TODO(dsnet): Reduce memory allocations by caching slices.
}

// Init initializes the moveToFront codec. The dict must contain all of the
// symbols in the alphabet used in future operations. A copy of the input dict
// will be made so that it will not be mutated.
func (m *moveToFront) Init(dict []uint8) {
	if len(dict) > len(m.dictBuf) {
		panic("alphabet too large")
	}
	copy(m.dictBuf[:], dict)
	m.dictLen = len(dict)
}

func (m *moveToFront) Encode(vals []byte) (idxs []uint8, runs []uint32) {
	dict := m.dictBuf[:m.dictLen]

	var lastCnt *uint32
	for _, val := range vals {
		// Normal move-to-front transform.
		var idx uint8 // Reverse lookup idx in dict
		for di, dv := range dict {
			if dv == val {
				idx = uint8(di)
				break
			}
		}
		copy(dict[1:], dict[:idx])
		dict[0] = val

		// Run-length encoding augmentation.
		if idx == 0 {
			if lastCnt == nil {
				idxs = append(idxs, 0)
				runs = append(runs, 0)
				lastCnt = &runs[len(runs)-1]
			}
			(*lastCnt)++
		} else {
			idxs = append(idxs, idx)
			lastCnt = nil
		}
	}
	return idxs, runs
}

func (m *moveToFront) Decode(idxs []uint8, runs []uint32) (vals []byte) {
	dict := m.dictBuf[:m.dictLen]

	var i int
	for _, idx := range idxs {
		// Normal move-to-front transform.
		val := dict[idx] // Forward lookup val in dict
		copy(dict[1:], dict[:idx])
		dict[0] = val

		// Run-length encoding augmentation.
		if idx == 0 {
			rep := int(runs[i])
			i++
			for j := 0; j < rep; j++ {
				vals = append(vals, val)
			}
		} else {
			vals = append(vals, val)
		}
	}
	return vals
}

// For the RLE encoding that is applied after MTF, a bijective base-2 numeration
// is used. This is a variable length code, so the length of the input effects
// the value of the output.
//
// To save space, the RLE encoding is stored in a single uint32, where the lower
// 5-bits are used for the bit-length, the upper 27-bits are for the RLE code
// itself. RUNA is represented by a 0; RUNB is represented by a 1. The bits
// are packed in LE order; that is, the least significant bit is in the LSB
// position of the integer. This encoding has a maximum size of ~256MiB.
type runCode uint32

func (v runCode) Encode() (x uint32) {
	var n int
	if v > 0 {
		for rep := v - 1; ; rep = (rep - 2) / 2 {
			if x >>= 1; rep&1 > 0 {
				x |= 0x80000000
			}
			n++
			if rep < 2 {
				break
			}
		}
		if n > 27 {
			return ^uint32(0) // Invalid value to cause problems later
		}
	}
	return (x >> uint(27-n)) | uint32(n)
}

func (v runCode) Decode() (x uint32) {
	repPwr := uint32(1)
	n := int(v & 0x1f)
	v >>= 5
	for i := 0; i < n; i++ {
		x += repPwr << (v & 1)
		repPwr <<= 1
		v >>= 1
	}
	if n > 27 {
		return ^uint32(0) // Invalid value to cause problems later
	}
	return x
}
