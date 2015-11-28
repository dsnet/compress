// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import "reflect"
import "testing"

func TestMoveToFront(t *testing.T) {
	var getDict = func(buf []byte) []uint8 {
		var dictMap [256]bool
		for _, b := range buf {
			dictMap[b] = true
		}
		var dictArr [256]uint8
		var i int
		for j, b := range dictMap {
			if b {
				dictArr[i] = uint8(j)
				i++
			}
		}
		return dictArr[:i]
	}

	var vectors = []struct {
		input   []byte
		outIdxs []uint8
		outRuns []uint32
	}{{
		input:   []byte{},
		outIdxs: []uint8{},
		outRuns: []uint32{},
	}, {
		input:   []byte{3},
		outIdxs: []uint8{0},
		outRuns: []uint32{1},
	}, {
		input:   []byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
		outIdxs: []uint8{0},
		outRuns: []uint32{10},
	}, {
		input:   []byte{9, 8, 7, 6, 5, 4, 3, 2, 1},
		outIdxs: []uint8{8, 8, 8, 8, 8, 8, 8, 8, 8},
		outRuns: []uint32{},
	}, {
		input:   []byte{42, 47, 42, 47, 42, 47, 42, 47, 42, 47, 42, 47},
		outIdxs: []uint8{0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		outRuns: []uint32{1},
	}, {
		input:   []byte{0, 5, 2, 3, 4, 4, 3, 1, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 5, 2, 3, 3},
		outIdxs: []uint8{0, 5, 3, 4, 5, 0, 1, 5, 3, 2, 0, 3, 0, 4, 3, 3, 0},
		outRuns: []uint32{1, 1, 5, 2, 1},
	}}

	mtf := new(moveToFront)
	for i, v := range vectors {
		dict := getDict(v.input)
		mtf.Init(dict)
		idxs, runs := mtf.Encode(append([]byte(nil), v.input...))
		mtf.Init(dict)
		input := mtf.Decode(idxs, runs)

		if !reflect.DeepEqual(input, v.input) && !(len(input) == 0 && len(v.input) == 0) {
			t.Errorf("test %d, input mismatch:\ngot  %v\nwant %v", i, input, v.input)
		}
		if !reflect.DeepEqual(idxs, v.outIdxs) && !(len(idxs) == 0 && len(v.outIdxs) == 0) {
			t.Errorf("test %d, output indexes mismatch:\ngot  %v\nwant %v", i, idxs, v.outIdxs)
		}
		if !reflect.DeepEqual(runs, v.outRuns) && !(len(runs) == 0 && len(v.outRuns) == 0) {
			t.Errorf("test %d, output runs mismatch:\ngot  %v\nwant %v", i, runs, v.outRuns)
		}
	}
}

func TestRunCode(t *testing.T) {
	var vectors = []struct {
		input  uint32
		output uint32
	}{
		{input: 0x00000000, output: 0x00000000},
		{input: 0x00000001, output: 0x00000001},
		{input: 0x00000002, output: 0x00000021},
		{input: 0x00000003, output: 0x00000002},
		{input: 0x00000004, output: 0x00000022},
		{input: 0x00000005, output: 0x00000042},
		{input: 0x00000006, output: 0x00000062},
		{input: 0x00000007, output: 0x00000003},
		{input: 0x00000008, output: 0x00000023},
		{input: 0x00000009, output: 0x00000043},
		{input: 0x0000000a, output: 0x00000063},
		{input: 0x0000000b, output: 0x00000083},
		{input: 0x0000000c, output: 0x000000a3},
		{input: 0x0000000d, output: 0x000000c3},
		{input: 0x0000000e, output: 0x000000e3},
		{input: 0x0000000f, output: 0x00000004},
		{input: 0x00000010, output: 0x00000024},
		{input: 0x00000011, output: 0x00000044},
		{input: 0x00000012, output: 0x00000064},
		{input: 0x00000013, output: 0x00000084},
		{input: 0x00000021, output: 0x00000045},
		{input: 0x0000015a, output: 0x00000b68},
		{input: 0x00001a8b, output: 0x0001518c},
		{input: 0x000cab82, output: 0x00957073},
		{input: 0x0ffffffe, output: 0xfffffffb},
		{input: 0x0fffffff, output: 0xffffffff},
		{input: 0xffffffff, output: 0xffffffff},
	}

	for i, v := range vectors {
		output := runCode(v.input).Encode()
		input := runCode(v.output).Decode()

		if input != v.input && output != 0xffffffff {
			t.Errorf("test %d, input mismatch: got 0x%08x, want 0x%08x", i, input, v.input)
		}
		if output != v.output && input != 0xffffffff {
			t.Errorf("test %d, output mismatch: got 0x%08x, want 0x%08x", i, output, v.output)
		}
	}
}
