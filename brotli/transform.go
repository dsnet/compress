// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package brotli

// These constants are defined in Appendix B of the RFC.
const (
	transformIdentity = iota
	transformUppercaseFirst
	transformUppercaseAll
	transformOmitFirst1
	transformOmitFirst2
	transformOmitFirst3
	transformOmitFirst4
	transformOmitFirst5
	transformOmitFirst6
	transformOmitFirst7
	transformOmitFirst8
	transformOmitFirst9
	transformOmitLast1
	transformOmitLast2
	transformOmitLast3
	transformOmitLast4
	transformOmitLast5
	transformOmitLast6
	transformOmitLast7
	transformOmitLast8
	transformOmitLast9
)

// This table is defined in Appendix B of the RFC.
var transformLUT = []struct {
	prefix    string
	transform int
	suffix    string
}{
	{"", transformIdentity, ""},
	{"", transformIdentity, " "},
	{" ", transformIdentity, " "},
	{"", transformOmitFirst1, ""},
	{"", transformUppercaseFirst, " "},
	{"", transformIdentity, " the "},
	{" ", transformIdentity, ""},
	{"s ", transformIdentity, " "},
	{"", transformIdentity, " of "},
	{"", transformUppercaseFirst, ""},
	{"", transformIdentity, " and "},
	{"", transformOmitFirst2, ""},
	{"", transformOmitLast1, ""},
	{", ", transformIdentity, " "},
	{"", transformIdentity, ", "},
	{" ", transformUppercaseFirst, " "},
	{"", transformIdentity, " in "},
	{"", transformIdentity, " to "},
	{"e ", transformIdentity, " "},
	{"", transformIdentity, "\""},
	{"", transformIdentity, "."},
	{"", transformIdentity, "\">"},
	{"", transformIdentity, "\n"},
	{"", transformOmitLast3, ""},
	{"", transformIdentity, "]"},
	{"", transformIdentity, " for "},
	{"", transformOmitFirst3, ""},
	{"", transformOmitLast2, ""},
	{"", transformIdentity, " a "},
	{"", transformIdentity, " that "},
	{" ", transformUppercaseFirst, ""},
	{"", transformIdentity, ". "},
	{".", transformIdentity, ""},
	{" ", transformIdentity, ", "},
	{"", transformOmitFirst4, ""},
	{"", transformIdentity, " with "},
	{"", transformIdentity, "'"},
	{"", transformIdentity, " from "},
	{"", transformIdentity, " by "},
	{"", transformOmitFirst5, ""},
	{"", transformOmitFirst6, ""},
	{" the ", transformIdentity, ""},
	{"", transformOmitLast4, ""},
	{"", transformIdentity, ". The "},
	{"", transformUppercaseAll, ""},
	{"", transformIdentity, " on "},
	{"", transformIdentity, " as "},
	{"", transformIdentity, " is "},
	{"", transformOmitLast7, ""},
	{"", transformOmitLast1, "ing "},
	{"", transformIdentity, "\n\t"},
	{"", transformIdentity, ":"},
	{" ", transformIdentity, ". "},
	{"", transformIdentity, "ed "},
	{"", transformOmitFirst9, ""},
	{"", transformOmitFirst7, ""},
	{"", transformOmitLast6, ""},
	{"", transformIdentity, "("},
	{"", transformUppercaseFirst, ", "},
	{"", transformOmitLast8, ""},
	{"", transformIdentity, " at "},
	{"", transformIdentity, "ly "},
	{" the ", transformIdentity, " of "},
	{"", transformOmitLast5, ""},
	{"", transformOmitLast9, ""},
	{" ", transformUppercaseFirst, ", "},
	{"", transformUppercaseFirst, "\""},
	{".", transformIdentity, "("},
	{"", transformUppercaseAll, " "},
	{"", transformUppercaseFirst, "\">"},
	{"", transformIdentity, "=\""},
	{" ", transformIdentity, "."},
	{".com/", transformIdentity, ""},
	{" the ", transformIdentity, " of the "},
	{"", transformUppercaseFirst, "'"},
	{"", transformIdentity, ". This "},
	{"", transformIdentity, ","},
	{".", transformIdentity, " "},
	{"", transformUppercaseFirst, "("},
	{"", transformUppercaseFirst, "."},
	{"", transformIdentity, " not "},
	{" ", transformIdentity, "=\""},
	{"", transformIdentity, "er "},
	{" ", transformUppercaseAll, " "},
	{"", transformIdentity, "al "},
	{" ", transformUppercaseAll, ""},
	{"", transformIdentity, "='"},
	{"", transformUppercaseAll, "\""},
	{"", transformUppercaseFirst, ". "},
	{" ", transformIdentity, "("},
	{"", transformIdentity, "ful "},
	{" ", transformUppercaseFirst, ". "},
	{"", transformIdentity, "ive "},
	{"", transformIdentity, "less "},
	{"", transformUppercaseAll, "'"},
	{"", transformIdentity, "est "},
	{" ", transformUppercaseFirst, "."},
	{"", transformUppercaseAll, "\">"},
	{" ", transformIdentity, "='"},
	{"", transformUppercaseFirst, ","},
	{"", transformIdentity, "ize "},
	{"", transformUppercaseAll, "."},
	{"\xc2\xa0", transformIdentity, ""},
	{" ", transformIdentity, ","},
	{"", transformUppercaseFirst, "=\""},
	{"", transformUppercaseAll, "=\""},
	{"", transformIdentity, "ous "},
	{"", transformUppercaseAll, ", "},
	{"", transformUppercaseFirst, "='"},
	{" ", transformUppercaseFirst, ","},
	{" ", transformUppercaseAll, "=\""},
	{" ", transformUppercaseAll, ", "},
	{"", transformUppercaseAll, ","},
	{"", transformUppercaseAll, "("},
	{"", transformUppercaseAll, ". "},
	{" ", transformUppercaseAll, "."},
	{"", transformUppercaseAll, "='"},
	{" ", transformUppercaseAll, ". "},
	{" ", transformUppercaseFirst, "=\""},
	{" ", transformUppercaseAll, "='"},
	{" ", transformUppercaseFirst, "='"},
}
