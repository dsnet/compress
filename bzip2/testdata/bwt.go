package main

import "fmt"
import "crypto/md5"
import "io/ioutil"
import "github.com/larzconwell/bzip2/internal/bwt"

func main() {
	for _, f := range []string{"binary.bin", "digits.txt", "huffman.txt", "random.bin", "repeats.bin", "twain.txt", "zeros.bin"} {
		arr1, err := ioutil.ReadFile(f)
		if err != nil {
			panic(err)
		}
		arr2 := make([]byte, len(arr1))
		p := bwt.Transform(arr2, arr1)
		pf("%s %d %x\n", f, p, md5.Sum(arr2))

		decodeBWT(arr2, p)
		if string(arr2) != string(arr1) {
			pl("WTF", f)
		}
	}
}

var pl, pf = fmt.Println, fmt.Printf

func decodeBWT(buf []byte, ptr int) {
	if len(buf) == 0 {
		return
	}

	var c [256]int
	for _, v := range buf {
		c[v]++
	}

	var sum int
	for i, v := range c {
		sum += v
		c[i] = sum - v
	}

	tt := make([]int, len(buf))
	for i := range buf {
		b := buf[i]
		tt[c[b]] |= i
		c[b]++
	}

	buf2 := make([]byte, len(buf))
	tPos := tt[ptr]
	for i := range tt {
		buf2[i] = buf[tPos]
		tPos = tt[tPos]
	}
	copy(buf, buf2)
}
