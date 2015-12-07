// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"testing"
)

func TestBurrowsWheelerTransform(t *testing.T) {
	var loadFile = func(path string) string {
		buf, err := ioutil.ReadFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return string(buf)
	}
	var ss = func(s string) string {
		const limit = 256
		if len(s) > limit {
			return fmt.Sprintf("%q...", s[:limit])
		}
		return fmt.Sprintf("%q", s)
	}

	var vectors = []struct {
		input  string // The input test string
		output string // Expected output string after BWT (skip if empty)
		chksum string // Expected output MD5 checksum (skip if empty)
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
	}, {
		input:  loadFile("testdata/binary.bin"),
		chksum: "bfd78db025b6842fc5261657123409c2",
		ptr:    123410,
	}, {
		input:  loadFile("testdata/digits.txt"),
		chksum: "9919ff629799fffb771805dbb41164e5",
		ptr:    20151,
	}, {
		input:  loadFile("testdata/huffman.txt"),
		chksum: "956c008d3e3485103031b1ce92f58094",
		ptr:    79034,
	}, {
		input:  loadFile("testdata/random.bin"),
		chksum: "93bfbaa5017b861da2d405815a8036bb",
		ptr:    1782,
	}, {
		input:  loadFile("testdata/repeats.bin"),
		chksum: "96b0d6d4df9dfe869b5b8fdbe70a6e7f",
		ptr:    131563,
	}, {
		input:  loadFile("testdata/twain.txt"),
		chksum: "123105351d920f489cc9941dd48a8c4e",
		ptr:    47,
	}, {
		input:  loadFile("testdata/zeros.bin"),
		chksum: "ec87a838931d4d5d2e94a04644788a55",
		ptr:    262143,
	}}

	bwt := new(burrowsWheelerTransform)
	for i, v := range vectors {
		b := []byte(v.input)
		p := bwt.Encode(b)
		output := string(b)
		chksum := fmt.Sprintf("%x", md5.Sum(b))
		bwt.Decode(b, p)
		input := string(b)

		if input != v.input {
			t.Errorf("test %d, input mismatch:\ngot  %v\nwant %v", i, ss(input), ss(v.input))
		}
		if output != v.output && v.output != "" {
			t.Errorf("test %d, output mismatch:\ngot  %v\nwant %v", i, ss(output), ss(v.output))
		}
		if chksum != v.chksum && v.chksum != "" {
			t.Errorf("test %d, checksum mismatch:\ngot  %s\nwant %s", i, chksum, v.chksum)
		}
		if p != v.ptr {
			t.Errorf("test %d, pointer mismatch: got %d, want %d", i, p, v.ptr)
		}
	}
}
