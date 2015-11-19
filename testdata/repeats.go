// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build ignore

// Generates repeats.bin. This test file heavily favors LZ77 based compression
// since a large bulk of its data is a copy from some distance ago. Also, since
// the source data is mostly random, prefix encoding does not benefit as much.
package main

import "io/ioutil"
import "math/rand"

const (
	name = "repeats.bin"
	size = 1 << 18
)

func main() {
	var b []byte
	var r = rand.New(rand.NewSource(0))

	randLen := func() (l int) {
		p := r.Float32()
		switch {
		case p <= 0.15: // 4..8
			l = 4 + r.Int()%4
		case p <= 0.30: // 8..16
			l = 8 + r.Int()%8
		case p <= 0.45: // 16..32
			l = 16 + r.Int()%16
		case p <= 0.60: // 32..64
			l = 32 + r.Int()%32
		case p <= 0.75: // 64..128
			l = 64 + r.Int()%64
		case p <= 0.90: // 128..256
			l = 128 + r.Int()%128
		case p <= 1.0: // 256..512
			l = 256 + r.Int()%256
		}
		return l
	}

	randDist := func() (d int) {
		for d == 0 || d > len(b) {
			p := r.Float32()
			switch {
			case p <= 0.1: // 1..2
				d = 1 + r.Int()%1
			case p <= 0.2: // 2..4
				d = 2 + r.Int()%2
			case p <= 0.3: // 4..8
				d = 4 + r.Int()%4
			case p <= 0.4: // 8..16
				d = 8 + r.Int()%8
			case p <= 0.5: // 16..32
				d = 16 + r.Int()%16
			case p <= 0.55: // 32..64
				d = 32 + r.Int()%32
			case p <= 0.60: // 64..128
				d = 64 + r.Int()%64
			case p <= 0.65: // 128..256
				d = 128 + r.Int()%128
			case p <= 0.70: // 256..512
				d = 256 + r.Int()%256
			case p <= 0.75: // 512..1024
				d = 512 + r.Int()%512
			case p <= 0.80: // 1024..2048
				d = 1024 + r.Int()%1024
			case p <= 0.85: // 2048..4096
				d = 2048 + r.Int()%2048
			case p <= 0.90: // 4096..8192
				d = 4096 + r.Int()%4096
			case p <= 0.95: // 8192..16384
				d = 8192 + r.Int()%8192
			case p <= 1.00: // 16384..32768
				d = 16384 + r.Int()%16384
			}
		}
		return d
	}

	writeRand := func(l int) {
		for i := 0; i < l; i++ {
			b = append(b, byte(r.Int()))
		}
	}

	writeCopy := func(d, l int) {
		for i := 0; i < l; i++ {
			b = append(b, b[len(b)-d])
		}
	}

	writeRand(randLen())
	for len(b) < size {
		p := r.Float32()
		switch {
		case p <= 0.1:
			// Generate random new data.
			writeRand(randLen())
		case p <= 0.9:
			// Write a long distance copy.
			d, l := randDist(), randLen()
			for d <= l {
				d, l = randDist(), randLen()
			}
			writeCopy(d, l)
		case p <= 1.0:
			// Write a possibly short distance copy.
			writeCopy(randDist(), randLen())
		}
	}

	if err := ioutil.WriteFile(name, b[:size], 0664); err != nil {
		panic(err)
	}
}
