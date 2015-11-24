// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// ====================================================
// Copyright (c) 2008-2010 Yuta Mori All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person
// obtaining a copy of this software and associated documentation
// files (the "Software"), to deal in the Software without
// restriction, including without limitation the rights to use,
// copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following
// conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
// OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
// HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
// WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.
// ====================================================

package sais

func getCounts_int(T []int, C []int, n, k int) {
	var i int
	for i = 0; i < k; i++ {
		C[i] = 0
	}
	for i = 0; i < n; i++ {
		C[T[i]]++
	}
}

func getBuckets_int(C, B []int, k int, end bool) {
	var i, sum int
	if end {
		for i = 0; i < k; i++ {
			sum += C[i]
			B[i] = sum
		}
	} else {
		for i = 0; i < k; i++ {
			sum += C[i]
			B[i] = sum - C[i]
		}
	}
}

func sortLMS1_int(T []int, SA, C, B []int, n, k int) {
	var b, i, j int
	var c0, c1 int

	// Compute SAl.
	if &C[0] == &B[0] {
		getCounts_int(T, C, n, k)
	}
	getBuckets_int(C, B, k, false) // Find starts of buckets
	j = n - 1
	c1 = int(T[j])
	b = B[c1]
	j--
	if int(T[j]) < c1 {
		SA[b] = ^j
	} else {
		SA[b] = j
	}
	b++
	for i = 0; i < n; i++ {
		if j = SA[i]; j > 0 {
			if c0 = int(T[j]); c0 != c1 {
				B[c1] = b
				c1 = c0
				b = B[c1]
			}
			j--
			if int(T[j]) < c1 {
				SA[b] = ^j
			} else {
				SA[b] = j
			}
			b++
			SA[i] = 0
		} else if j < 0 {
			SA[i] = ^j
		}
	}

	// Compute SAs.
	if &C[0] == &B[0] {
		getCounts_int(T, C, n, k)
	}
	getBuckets_int(C, B, k, true) // Find ends of buckets
	c1 = 0
	b = B[c1]
	for i = n - 1; i >= 0; i-- {
		if j = SA[i]; j > 0 {
			if c0 = int(T[j]); c0 != c1 {
				B[c1] = b
				c1 = c0
				b = B[c1]
			}
			j--
			b--
			if int(T[j]) > c1 {
				SA[b] = ^(j + 1)
			} else {
				SA[b] = j
			}
			SA[i] = 0
		}
	}
}

func postProcLMS1_int(T []int, SA []int, n, m int) int {
	var i, j, p, q, plen, qlen, name int
	var c0, c1 int
	var diff bool

	// Compact all the sorted substrings into the first m items of SA.
	// 2*m must be not larger than n (provable).
	for i = 0; SA[i] < 0; i++ {
		SA[i] = ^SA[i]
	}
	if i < m {
		for j, i = i, i+1; ; i++ {
			if p = SA[i]; p < 0 {
				SA[j] = ^p
				j++
				SA[i] = 0
				if j == m {
					break
				}
			}
		}
	}

	// Store the length of all substrings.
	i = n - 1
	j = n - 1
	c0 = int(T[n-1])
	for {
		c1 = c0
		if i--; i < 0 {
			break
		}
		if c0 = int(T[i]); c0 < c1 {
			break
		}
	}
	for i >= 0 {
		for {
			c1 = c0
			if i--; i < 0 {
				break
			}
			if c0 = int(T[i]); c0 > c1 {
				break
			}
		}
		if i >= 0 {
			SA[m+((i+1)>>1)] = j - i
			j = i + 1
			for {
				c1 = c0
				if i--; i < 0 {
					break
				}
				if c0 = int(T[i]); c0 < c1 {
					break
				}
			}
		}
	}

	// Find the lexicographic names of all substrings.
	name = 0
	qlen = 0
	for i, q = 0, n; i < m; i++ {
		p = SA[i]
		plen = SA[m+(p>>1)]
		diff = true
		if (plen == qlen) && ((q + plen) < n) {
			for j = 0; (j < plen) && (T[p+j] == T[q+j]); j++ {
			}
			if j == plen {
				diff = false
			}
		}
		if diff {
			name++
			q = p
			qlen = plen
		}
		SA[m+(p>>1)] = name
	}
	return name
}

func sortLMS2_int(T []int, SA, C, B, D []int, n, k int) {
	var b, i, j, t, d int
	var c0, c1 int

	// Compute SAl.
	getBuckets_int(C, B, k, false) // Find starts of buckets
	j = n - 1
	c1 = int(T[j])
	b = B[c1]
	j--
	if int(T[j]) < c1 {
		t = 1
	} else {
		t = 0
	}
	j += n
	if t&1 > 0 {
		SA[b] = ^j
	} else {
		SA[b] = j
	}
	b++
	for i, d = 0, 0; i < n; i++ {
		if j = SA[i]; j > 0 {
			if n <= j {
				d += 1
				j -= n
			}
			if c0 = int(T[j]); c0 != c1 {
				B[c1] = b
				c1 = c0
				b = B[c1]
			}
			j--
			t = int(c0) << 1
			if int(T[j]) < c1 {
				t |= 1
			}
			if D[t] != d {
				j += n
				D[t] = d
			}
			if t&1 > 0 {
				SA[b] = ^j
			} else {
				SA[b] = j
			}
			b++
			SA[i] = 0
		} else if j < 0 {
			SA[i] = ^j
		}
	}
	for i = n - 1; 0 <= i; i-- {
		if SA[i] > 0 {
			if SA[i] < n {
				SA[i] += n
				for j = i - 1; SA[j] < n; j-- {
				}
				SA[j] -= n
				i = j
			}
		}
	}

	// Compute SAs.
	getBuckets_int(C, B, k, true) // Find ends of buckets
	c1 = 0
	b = B[c1]
	for i, d = n-1, d+1; i >= 0; i-- {
		if j = SA[i]; j > 0 {
			if n <= j {
				d += 1
				j -= n
			}
			if c0 = int(T[j]); c0 != c1 {
				B[c1] = b
				c1 = c0
				b = B[c1]
			}
			j--
			t = int(c0) << 1
			if int(T[j]) > c1 {
				t |= 1
			}
			if D[t] != d {
				j += n
				D[t] = d
			}
			b--
			if t&1 > 0 {
				SA[b] = ^(j + 1)
			} else {
				SA[b] = j
			}
			SA[i] = 0
		}
	}
}

func postProcLMS2_int(SA []int, n, m int) int {
	var i, j, d, name int

	// Compact all the sorted LMS substrings into the first m items of SA.
	name = 0
	for i = 0; SA[i] < 0; i++ {
		j = ^SA[i]
		if n <= j {
			name += 1
		}
		SA[i] = j
	}
	if i < m {
		for d, i = i, i+1; ; i++ {
			if j = SA[i]; j < 0 {
				j = ^j
				if n <= j {
					name += 1
				}
				SA[d] = j
				d++
				SA[i] = 0
				if d == m {
					break
				}
			}
		}
	}
	if name < m {
		// Store the lexicographic names.
		for i, d = m-1, name+1; 0 <= i; i-- {
			if j = SA[i]; n <= j {
				j -= n
				d--
			}
			SA[m+(j>>1)] = d
		}
	} else {
		// Unset flags.
		for i = 0; i < m; i++ {
			if j = SA[i]; n <= j {
				j -= n
				SA[i] = j
			}
		}
	}
	return name
}

func induceSA_int(T []int, SA, C, B []int, n, k int) {
	var b, i, j int
	var c0, c1 int

	// Compute SAl.
	if &C[0] == &B[0] {
		getCounts_int(T, C, n, k)
	}
	getBuckets_int(C, B, k, false) // Find starts of buckets
	j = n - 1
	c1 = int(T[j])
	b = B[c1]
	if j > 0 && int(T[j-1]) < c1 {
		SA[b] = ^j
	} else {
		SA[b] = j
	}
	b++
	for i = 0; i < n; i++ {
		j = SA[i]
		SA[i] = ^j
		if j > 0 {
			j--
			if c0 = int(T[j]); c0 != c1 {
				B[c1] = b
				c1 = c0
				b = B[c1]
			}
			if j > 0 && int(T[j-1]) < c1 {
				SA[b] = ^j
			} else {
				SA[b] = j
			}
			b++
		}
	}

	// Compute SAs.
	if &C[0] == &B[0] {
		getCounts_int(T, C, n, k)
	}
	getBuckets_int(C, B, k, true) // Find ends of buckets
	c1 = 0
	b = B[c1]
	for i = n - 1; i >= 0; i-- {
		if j = SA[i]; j > 0 {
			j--
			if c0 = int(T[j]); c0 != c1 {
				B[c1] = b
				c1 = c0
				b = B[c1]
			}
			b--
			if (j == 0) || (int(T[j-1]) > c1) {
				SA[b] = ^j
			} else {
				SA[b] = j
			}
		} else {
			SA[i] = ^j
		}
	}
}

func computeSA_int(T []int, SA []int, fs, n, k int) {
	const (
		minBucketSize = 512
		sortLMS2Limit = 0x3fffffff
	)

	var C, B, D, RA []int
	var bo int // Offset of B relative to SA
	var b, i, j, m, p, q, name, newfs int
	var c0, c1 int
	var flags uint

	if k <= minBucketSize {
		C = make([]int, k)
		if k <= fs {
			bo = n + fs - k
			B = SA[bo:]
			flags = 1
		} else {
			B = make([]int, k)
			flags = 3
		}
	} else if k <= fs {
		C = SA[n+fs-k:]
		if k <= fs-k {
			bo = n + fs - 2*k
			B = SA[bo:]
			flags = 0
		} else if k <= 4*minBucketSize {
			B = make([]int, k)
			flags = 2
		} else {
			B = C
			flags = 8
		}
	} else {
		C = make([]int, k)
		flags = 4 | 8
	}
	if n <= sortLMS2Limit && 2 <= (n/k) {
		if flags&1 > 0 {
			if 2*k <= fs-k {
				flags |= 32
			} else {
				flags |= 16
			}
		} else if flags == 0 && 2*k <= (fs-2*k) {
			flags |= 32
		}
	}

	// Stage 1: Reduce the problem by at least 1/2.
	// Sort all the LMS-substrings.
	getCounts_int(T, C, n, k)
	getBuckets_int(C, B, k, true) // Find ends of buckets
	for i = 0; i < n; i++ {
		SA[i] = 0
	}
	b = -1
	i = n - 1
	j = n
	m = 0
	c0 = int(T[n-1])
	for {
		c1 = c0
		if i--; i < 0 {
			break
		}
		if c0 = int(T[i]); c0 < c1 {
			break
		}
	}
	for i >= 0 {
		for {
			c1 = c0
			if i--; i < 0 {
				break
			}
			if c0 = int(T[i]); c0 > c1 {
				break
			}
		}
		if i >= 0 {
			if b >= 0 {
				SA[b] = j
			}
			B[c1]--
			b = B[c1]
			j = i
			m++
			for {
				c1 = c0
				if i--; i < 0 {
					break
				}
				if c0 = int(T[i]); c0 < c1 {
					break
				}
			}
		}
	}

	if m > 1 {
		if flags&(16|32) > 0 {
			if flags&16 > 0 {
				D = make([]int, 2*k)
			} else {
				D = SA[bo-2*k:]
			}
			B[T[j+1]]++
			for i, j = 0, 0; i < k; i++ {
				j += C[i]
				if B[i] != j {
					SA[B[i]] += n
				}
				D[i] = 0
				D[i+k] = 0
			}
			sortLMS2_int(T, SA, C, B, D, n, k)
			name = postProcLMS2_int(SA, n, m)

		} else {
			sortLMS1_int(T, SA, C, B, n, k)
			name = postProcLMS1_int(T, SA, n, m)
		}
	} else if m == 1 {
		SA[b] = j + 1
		name = 1
	} else {
		name = 0
	}

	// Stage 2: Solve the reduced problem.
	// Recurse if names are not yet unique.
	if name < m {
		newfs = n + fs - 2*m
		if flags&(1|4|8) == 0 {
			if k+name <= newfs {
				newfs -= k
			} else {
				flags |= 8
			}
		}
		RA = SA[m+newfs:]
		for i, j = m+(n>>1)-1, m-1; m <= i; i-- {
			if SA[i] != 0 {
				RA[j] = SA[i] - 1
				j--
			}
		}
		computeSA_int(RA, SA, newfs, m, name)

		i = n - 1
		j = m - 1
		c0 = int(T[n-1])
		for {
			c1 = c0
			if i--; i < 0 {
				break
			}
			if c0 = int(T[i]); c0 < c1 {
				break
			}
		}
		for i >= 0 {
			for {
				c1 = c0
				if i--; i < 0 {
					break
				}
				if c0 = int(T[i]); c0 > c1 {
					break
				}
			}
			if i >= 0 {
				RA[j] = i + 1
				j--
				for {
					c1 = c0
					if i--; i < 0 {
						break
					}
					if c0 = int(T[i]); c0 < c1 {
						break
					}
				}
			}
		}
		for i = 0; i < m; i++ {
			SA[i] = RA[SA[i]]
		}
		if flags&4 > 0 {
			B = make([]int, k)
			C = B
		}
		if flags&2 > 0 {
			B = make([]int, k)
		}
	}

	// Stage 3: Induce the result for the original problem.
	if flags&8 > 0 {
		getCounts_int(T, C, n, k)
	}
	// Put all left-most S characters into their buckets.
	if m > 1 {
		getBuckets_int(C, B, k, true) // Find ends of buckets
		i = m - 1
		j = n
		p = SA[m-1]
		c1 = int(T[p])
		for {
			c0 = c1
			q = B[c0]
			for q < j {
				j--
				SA[j] = 0
			}
			for {
				j--
				SA[j] = p
				if i--; i < 0 {
					break
				}
				p = SA[i]
				if c1 = int(T[p]); c1 != c0 {
					break
				}
			}
			if i < 0 {
				break
			}
		}
		for j > 0 {
			j--
			SA[j] = 0
		}
	}
	induceSA_int(T, SA, C, B, n, k)
}
