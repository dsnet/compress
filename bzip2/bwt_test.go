// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"testing"
)

func TestBurrowsWheelerTransform(t *testing.T) {
	var vectors = []struct {
		input  string // The input test string
		output string // Expected output string after BWT (skip if empty)
		ptr    int    // The BWT origin pointer
	}{{
		input:  "",
		output: "",
		ptr:    -1,
	}, {
		input:  "Hello, world!",
		output: ",do!lHrellwo ",
		ptr:    3,
	}, {
		input:  "SIX.MIXED.PIXIES.SIFT.SIXTY.PIXIE.DUST.BOXES",
		output: "TEXYDST.E.IXIXIXXSSMPPS.B..E.S.EUSFXDIIOIIIT",
		ptr:    29,
	}, {
		input:  "0123456789",
		output: "9012345678",
		ptr:    0,
	}, {
		input:  "9876543210",
		output: "1234567890",
		ptr:    9,
	}, {
		input:  "The quick brown fox jumped over the lazy dog.",
		output: "kynxederg.l ie hhpv otTu c uwd rfm eb qjoooza",
		ptr:    9,
	}, {
		input: "Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Mary had a little lamb, its fleece was white as snow" +
			"Nary had a little lamb, its fleece was white as snow",
		output: "dddddddddeeeeeeeeesssssssssyyyyyyyyy,,,,,,,,,eeeeeee" +
			"eeaaaaaaaaassssssssseeeeeeeeesssssssssbbbbbbbbbwwwww" +
			"wwww         hhhhhhhhhlllllllllNMMMMMMMM         www" +
			"wwwwwwmmmmmmmmmeeeeeeeeeaaaaaaaaatttttttttlllllllllc" +
			"cccccccceeeeeeeeelllllllll                  wwwwwwww" +
			"whhhhhhhhh         lllllllll         tttttttttffffff" +
			"fff         aaaaaaaaasssssssssnnnnnnnnnaaaaaaaaatttt" +
			"tttttaaaaaaaaaaaaaaaaaa         iiiiiiiiitttttttttii" +
			"iiiiiiiiiiiiiiiiooooooooo                  rrrrrrrrr",
		ptr: 99,
	}, {
		input: "AGCTTTTCATTCTGACTGCAACGGGCAATATGTCTCTGTGTGGATTAAAAAAAGAGTCTCTGAC" +
			"AGCAGCTTCTGAACTGGTTACCTGCCGTGAGTAAATTAAAATTTTATTGACTTAGGTCACTAAA" +
			"TACTTTAACCAATATAGGCATAGCGCACAGACAGATAAAAATTACAGAGTACACAACATCCATG" +
			"AAACGCATTAGCACCACCATTACCACCACCATCACCACCACCATCACCATTACCATTACCACAG" +
			"GTAACGGTGCGGGCTGACGCGTACAGGAAACACAGAAAAAAGCCCGCACCTGACAGTGCGGGCT" +
			"TTTTTTTCGACCAAAGGTAACGAGGTAACAACCATGCGAGTGTTGAAGTTCGGCGGTACATCAG" +
			"TGGCAAATGCAGAACGTTTTCTGCGGGTTGCCGATATTCTGGAAAGCAATGCCAGGCAGGGGCA",
		output: "TAGAATAAATGGAGACTCTAATACTCTACTGGAAACAGACCACAAACATACCTGGTCGTAGATT" +
			"CCCCCCATCCCTAAGAAACGAGTCCCCACATCATCACCTCGACTGGGCCGAGACTAAGCCCCCA" +
			"ACTGAACCCCCTTACGAAGGCGGAAGCTCCGCCCTGTAGAAAAGACGAATGCCAACCCCCGTAA" +
			"AAAAAAGAATAAAAGGCGAATAGCGCAATAGGGGAGCAATTTTCGTACTTATAGAGGAGTGATT" +
			"ATTCTTTCTAACACGGTGGACACTAGGCTATTTATTTGCGAAGATTTGGAACGGGCCCACAAAC" +
			"ACTGAGGGACGGATCGATATAGATGCTATCGGTGGGTGGTTTTATAATAAATAAGATATTGGTC" +
			"TTTCACTCCCCTGCAATCAGGCCGGCAGCGAATAAAAGACTTTGCATAGAGCTTTTACTGTTTC",
		ptr: 99,
	}}

	bwt := new(burrowsWheelerTransform)
	for i, v := range vectors {
		b := []byte(v.input)
		p := bwt.Encode(b)
		output := string(b)
		b = []byte(v.output)
		bwt.Decode(b, p)
		input := string(b)

		if input != v.input {
			t.Errorf("test %d, input mismatch:\ngot  %q\nwant %q", i, input, v.input)
		}
		if output != v.output {
			t.Errorf("test %d, output mismatch:\ngot  %q\nwant %q", i, output, v.output)
		}
		if p != v.ptr {
			t.Errorf("test %d, pointer mismatch: got %d, want %d", i, p, v.ptr)
		}
	}
}
