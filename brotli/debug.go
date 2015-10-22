package brotli

import "os"
import "fmt"
import "strings"

func printLUTs() {
	print := func(name string, obj interface{}) {
		var body string
		if bs, ok := obj.([]uint8); ok && len(bs) >= 256 {
			var ss []string
			ss = append(ss, "{\n")
			var s string
			for i, b := range bs {
				s += fmt.Sprintf("%02x ", b)
				if i%16 == 15 || i+1 == len(bs) {
					ss = append(ss, "\t"+s+"\n")
					s = ""
				}
				if i%256 == 255 && (i+1 != len(bs)) {
					ss = append(ss, "\n")
				}
			}
			ss = append(ss, "}\n")
			body = strings.Join(ss, "")
		} else {
			body = fmt.Sprintf("%v", obj)
		}
		body = strings.TrimSpace(body)
		fmt.Fprintf(os.Stderr, "var %s %T = %v\n\n", name, obj, body)
	}

	// Common LUTs.
	print("reverseLUT", reverseLUT)

	// Context LUTs.
	print("contextP1LUT", contextP1LUT)
	print("contextP2LUT", contextP2LUT)

	// Static dictionary LUTs.
	print("dictBitSizes", dictBitSizes)
	print("dictSizes", dictSizes)
	print("dictOffsets", dictOffsets)

	// Prefix LUTs.
	print("simpleLens1", simpleLens1)
	print("simpleLens2", simpleLens2)
	print("simpleLens3", simpleLens3)
	print("simpleLens4a", simpleLens4a)
	print("simpleLens4b", simpleLens4b)
	print("codeLens", codeLens)

	print("decCodeLens", decCodeLens)
	print("decMaxRLE", decMaxRLE)
	print("decWinBits", decWinBits)
	print("decCounts", decCounts)

	print("encCodeLens", encCodeLens)
	print("encMaxRLE", encMaxRLE)
	print("encWinBits", encWinBits)
	print("encCounts", encCounts)
}
