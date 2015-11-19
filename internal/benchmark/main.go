// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build ignore

package main

import "os"
import "fmt"
import "flag"
import "sort"
import "math"
import "regexp"
import "strings"
import "go/build"
import "github.com/dsnet/golib/strconv"
import "github.com/dsnet/compress/internal/benchmark"

// By default, the benchmark tool will look for test data in this "package".
const testPkg = "github.com/dsnet/compress/testdata"

// The decompression speed benchmark works by decompressing some pre-compressed
// data. In order for the benchmarks to be consistent, the same encoder should
// be used to generate the pre-compressed data for all the trials.
//
// encRefs defines the priority order for which encoders to choose first as the
// reference compressor. If no compressor is found for any of the listed codecs,
// then a random encoder will be chosen.
var encRefs = []string{"std", "cgo", "ds"}

const (
	defaultTests  = "encRate,decRate,ratio"
	defaultFiles  = "zeros.bin,random.bin,binary.bin,repeats.bin,huffman.txt,digits.txt,twain.txt"
	defaultLevels = "1,5,9"
	defaultSizes  = "1e4,1e5,1e6"
)

var (
	fmtToEnum = map[string]int{
		"fl":  benchmark.FormatFlate,
		"bz2": benchmark.FormatBZ2,
		"xz":  benchmark.FormatXZ,
		"br":  benchmark.FormatBrotli,
	}
	enumToFmt = map[int]string{
		benchmark.FormatFlate:  "fl",
		benchmark.FormatBZ2:    "bz2",
		benchmark.FormatXZ:     "xz",
		benchmark.FormatBrotli: "br",
	}
	testToEnum = map[string]int{
		"encRate": benchmark.TestEncodeRate,
		"decRate": benchmark.TestDecodeRate,
		"ratio":   benchmark.TestCompressRatio,
	}
	enumToTest = map[int]string{
		benchmark.TestEncodeRate:    "encRate",
		benchmark.TestDecodeRate:    "decRate",
		benchmark.TestCompressRatio: "ratio",
	}
)

func defaultCodecs() string {
	m := make(map[string]bool)
	for _, v := range benchmark.Encoders {
		for k := range v {
			m[k] = true
		}
	}
	for _, v := range benchmark.Decoders {
		for k := range v {
			m[k] = true
		}
	}
	hasDS := m["std"]
	delete(m, "std")
	var s []string
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	if hasDS {
		s = append([]string{"std"}, s...) // Ensure "std" always appears first
	}
	return strings.Join(s, ",")
}

func defaultFormats() string {
	m := make(map[int]bool)
	for k := range benchmark.Encoders {
		m[k] = true
	}
	for k := range benchmark.Decoders {
		m[k] = true
	}
	s := make([]string, 0, len(m))
	for k := range m {
		if _, ok := enumToFmt[k]; !ok {
			panic("unknown format")
		}
		s = append(s, enumToFmt[k])
	}
	sort.Strings(s)
	return strings.Join(s, ",")
}

func defaultPaths() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	pkg, err := build.Import(testPkg, "", build.FindOnly)
	if err != nil {
		return cwd
	}
	return pkg.Dir + ":" + cwd
}

func main() {
	// Setup flag arguments.
	f0 := flag.String("tests", defaultTests, "List of different benchmark tests")
	f1 := flag.String("files", defaultFiles, "List of input files to benchmark")
	f2 := flag.String("levels", defaultLevels, "List of compression levels to benchmark")
	f3 := flag.String("sizes", defaultSizes, "List of input sizes to benchmark")
	f4 := flag.String("codecs", defaultCodecs(), "List of codecs to benchmark")
	f5 := flag.String("fmts", defaultFormats(), "List of formats to benchmark")
	f6 := flag.String("paths", defaultPaths(), "List of paths to search for test files")
	flag.Parse()

	// Parse the flag arguments.
	var sep = regexp.MustCompile("[,:]")
	var files, codecs, paths []string
	var tests, levels, sizes, fmts []int
	files = sep.Split(*f1, -1)
	codecs = sep.Split(*f4, -1)
	paths = sep.Split(*f6, -1)
	for _, s := range sep.Split(*f0, -1) {
		if _, ok := testToEnum[s]; !ok {
			panic("invalid test")
		}
		tests = append(tests, testToEnum[s])
	}
	for _, s := range sep.Split(*f2, -1) {
		lvl, err := strconv.ParsePrefix(s, strconv.AutoParse)
		if err != nil {
			panic("invalid level")
		}
		levels = append(levels, int(lvl))
	}
	for _, s := range sep.Split(*f3, -1) {
		var size int
		if nf, err := strconv.ParsePrefix(s, strconv.AutoParse); err == nil {
			size = int(nf)
		}
		sizes = append(sizes, size)
	}
	for _, s := range sep.Split(*f5, -1) {
		if _, ok := fmtToEnum[s]; !ok {
			panic("invalid format")
		}
		fmts = append(fmts, fmtToEnum[s])
	}

	benchmark.Paths = paths
	runBenchmarks(files, codecs, tests, levels, sizes, fmts)
}

func runBenchmarks(files, codecs []string, tests, levels, sizes, fmts []int) {
	for _, f := range fmts {
		// Get lists of encoders and decoders that exist.
		var encs, decs []string
		for _, c := range codecs {
			if _, ok := benchmark.Encoders[f][c]; ok {
				encs = append(encs, c)
			}
		}
		for _, c := range codecs {
			if _, ok := benchmark.Decoders[f][c]; ok {
				decs = append(decs, c)
			}
		}

		for _, t := range tests {
			var results [][]benchmark.Result
			var names, codecs []string
			var title, suffix string

			// Check that we can actually do this benchmark.
			fmt.Printf("BENCHMARK: %s:%s\n", enumToFmt[f], enumToTest[t])
			if len(encs) == 0 {
				fmt.Println("\tSKIP: There are no encoders available.\n")
				continue
			}
			if len(decs) == 0 && t == benchmark.TestDecodeRate {
				fmt.Println("\tSKIP: There are no decoders available.\n")
				continue
			}

			// Progress ticker.
			var cnt int
			tick := func() {
				total := len(codecs) * len(files) * len(levels) * len(sizes)
				pct := 100.0 * float64(cnt) / float64(total)
				fmt.Printf("\t[%6.2f%%] %d of %d\r", pct, cnt, total)
				cnt++
			}

			// Perform the benchmark. This may take some time.
			switch t {
			case benchmark.TestEncodeRate:
				codecs, title, suffix = encs, "MB/s", ""
				results, names = benchmark.BenchmarkEncoderSuite(f, encs, files, levels, sizes, tick)
			case benchmark.TestDecodeRate:
				ref := getReferenceEncoder(f)
				codecs, title, suffix = decs, "MB/s", ""
				results, names = benchmark.BenchmarkDecoderSuite(f, decs, files, levels, sizes, ref, tick)
			case benchmark.TestCompressRatio:
				codecs, title, suffix = encs, "ratio", "x"
				results, names = benchmark.BenchmarkRatioSuite(f, encs, files, levels, sizes, tick)
			default:
				panic("unknown test")
			}

			// Print all of the results.
			printResults(results, names, codecs, title, suffix)
			fmt.Println()
		}
		fmt.Println()
	}
}

func getReferenceEncoder(f int) benchmark.Encoder {
	for _, c := range encRefs {
		if enc, ok := benchmark.Encoders[f][c]; ok {
			return enc // Choose by priority
		}
	}
	for _, enc := range benchmark.Encoders[f] {
		return enc // Choose any random encoder
	}
	return nil // There are no encoders
}

func printResults(results [][]benchmark.Result, names, codecs []string, title, suffix string) {
	// Allocate result table.
	cells := make([][]string, 1+len(names))
	for i := range cells {
		cells[i] = make([]string, 1+2*len(codecs))
	}

	// Label the first row.
	cells[0][0] = "benchmark"
	for i, c := range codecs {
		cells[0][1+2*i] = c + " " + title
		cells[0][2+2*i] = "delta"
	}

	// Insert all rows.
	for j, row := range results {
		cells[1+j][0] = names[j]
		for i, r := range row {
			if r.R != 0 && !math.IsNaN(r.R) && !math.IsInf(r.R, 0) {
				cells[1+j][1+2*i] = fmt.Sprintf("%.2f", r.R) + suffix
			}
			if r.D != 0 && !math.IsNaN(r.D) && !math.IsInf(r.D, 0) {
				cells[1+j][2+2*i] = fmt.Sprintf("%.2f", r.D) + "x"
			}
		}
	}

	// Compute the maximum lengths.
	maxLens := make([]int, 1+2*len(codecs))
	for _, row := range cells {
		for i, s := range row {
			if maxLens[i] < len(s) {
				maxLens[i] = len(s)
			}
		}
	}

	// Print padded versions of all cells.
	for _, row := range cells {
		fmt.Print("\t")
		for i, s := range row {
			switch {
			case i == 0: // Column 0
				row[i] = s + strings.Repeat(" ", maxLens[i]-len(s))
			case i%2 == 1: // Column 1, 3, 5, 7, ...
				row[i] = strings.Repeat(" ", 6+maxLens[i]-len(s)) + s
			case i%2 == 0: // Column 2, 4, 6, 8, ...
				row[i] = strings.Repeat(" ", 2+maxLens[i]-len(s)) + s
			}
			fmt.Print(row[i])
		}
		fmt.Println()
	}
}
