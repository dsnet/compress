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

#include <assert.h>
#include <stdlib.h>
#include <stdio.h>

#define MINBUCKETSIZE 256
#define SAIS_LMSSORT2_LIMIT 0x3fffffff

#define SAIS_MYMALLOC(_num, _type) ((_type *)malloc((_num) * sizeof(_type)))
#define SAIS_MYFREE(_ptr, _num, _type) free((_ptr))
#define chr(_a) (cs == sizeof(int) ? ((int *)T)[(_a)] : ((unsigned char *)T)[(_a)])

static void getCounts(const void *T, int *C, int n, int k, int cs) {
	int i;
	for (i = 0; i < k; ++i) {
		C[i] = 0;
	}
	for (i = 0; i < n; ++i) {
		++C[chr(i)];
	}
}

static void getBuckets(const int *C, int *B, int k, int end) {
	int i, sum = 0;
	if (end) {
		for (i = 0; i < k; ++i) {
			sum += C[i];
			B[i] = sum;
		}
	} else {
		for (i = 0; i < k; ++i) {
			sum += C[i];
			B[i] = sum - C[i];
		}
	}
}

static void sortLMS1(const void *T, int *SA, int *C, int *B, int n, int k, int cs) {
	int *b, i, j;
	int c0, c1;

	// Compute SAl.
	if (C == B) {
		getCounts(T, C, n, k, cs);
	}
	getBuckets(C, B, k, 0); // Find starts of buckets
	j = n - 1;
	b = SA + B[c1 = chr(j)];
	--j;
	*b++ = (chr(j) < c1) ? ~j : j;
	for (i = 0; i < n; ++i) {
		if (0 < (j = SA[i])) {
			assert(chr(j) >= chr(j + 1));
			if ((c0 = chr(j)) != c1) {
				B[c1] = b - SA;
				b = SA + B[c1 = c0];
			}
			assert(i < (b - SA));
			--j;
			*b++ = (chr(j) < c1) ? ~j : j;
			SA[i] = 0;
		} else if (j < 0) {
			SA[i] = ~j;
		}
	}

	// Compute SAs.
	if (C == B) {
		getCounts(T, C, n, k, cs);
	}
	getBuckets(C, B, k, 1); // Find ends of buckets
	for (i = n - 1, b = SA + B[c1 = 0]; 0 <= i; --i) {
		if (0 < (j = SA[i])) {
			assert(chr(j) <= chr(j + 1));
			if ((c0 = chr(j)) != c1) {
				B[c1] = b - SA;
				b = SA + B[c1 = c0];
			}
			assert((b - SA) <= i);
			--j;
			*--b = (chr(j) > c1) ? ~(j + 1) : j;
			SA[i] = 0;
		}
	}
}

static int postProcLMS1(const void *T, int *SA, int n, int m, int cs) {
	int i, j, p, q, plen, qlen, name;
	int c0, c1;
	int diff;

	// Compact all the sorted substrings into the first m items of SA.
	// 2*m must be not larger than n (provable).
	assert(0 < n);
	for (i = 0; (p = SA[i]) < 0; ++i) {
		SA[i] = ~p;
		assert((i + 1) < n);
	}
	if (i < m) {
		for (j = i, ++i;; ++i) {
			assert(i < n);
			if ((p = SA[i]) < 0) {
				SA[j++] = ~p;
				SA[i] = 0;
				if (j == m) {
					break;
				}
			}
		}
	}

	// Store the length of all substrings.
	i = n - 1;
	j = n - 1;
	c0 = chr(n - 1);
	do {
		c1 = c0;
	}
	while ((0 <= --i) && ((c0 = chr(i)) >= c1));
	for (; 0 <= i;) {
		do {
			c1 = c0;
		}
		while ((0 <= --i) && ((c0 = chr(i)) <= c1));
		if (0 <= i) {
			SA[m + ((i + 1) >> 1)] = j - i;
			j = i + 1;
			do {
				c1 = c0;
			}
			while ((0 <= --i) && ((c0 = chr(i)) >= c1));
		}
	}

	// Find the lexicographic names of all substrings.
	for (i = 0, name = 0, q = n, qlen = 0; i < m; ++i) {
		p = SA[i], plen = SA[m + (p >> 1)], diff = 1;
		if ((plen == qlen) && ((q + plen) < n)) {
			for (j = 0; (j < plen) && (chr(p + j) == chr(q + j)); ++j) { }
			if (j == plen) {
				diff = 0;
			}
		}
		if (diff != 0) {
			++name, q = p, qlen = plen;
		}
		SA[m + (p >> 1)] = name;
	}

	return name;
}

static void LMSsort2(const void *T, int *SA, int *C, int *B, int *D, int n, int k, int cs) {
	int *b, i, j, t, d;
	int c0, c1;
	assert(C != B);

	// Compute SAl.
	getBuckets(C, B, k, 0); // Find starts of buckets
	j = n - 1;
	b = SA + B[c1 = chr(j)];
	--j;
	t = (chr(j) < c1);
	j += n;
	*b++ = (t & 1) ? ~j : j;
	for (i = 0, d = 0; i < n; ++i) {
		if (0 < (j = SA[i])) {
			if (n <= j) {
				d += 1;
				j -= n;
			}
			assert(chr(j) >= chr(j + 1));
			if ((c0 = chr(j)) != c1) {
				B[c1] = b - SA;
				b = SA + B[c1 = c0];
			}
			assert(i < (b - SA));
			--j;
			t = c0;
			t = (t << 1) | (chr(j) < c1);
			if (D[t] != d) {
				j += n;
				D[t] = d;
			}
			*b++ = (t & 1) ? ~j : j;
			SA[i] = 0;
		} else if (j < 0) {
			SA[i] = ~j;
		}
	}
	for (i = n - 1; 0 <= i; --i) {
		if (0 < SA[i]) {
			if (SA[i] < n) {
				SA[i] += n;
				for (j = i - 1; SA[j] < n; --j) { }
				SA[j] -= n;
				i = j;
			}
		}
	}

	// Compute SAs.
	getBuckets(C, B, k, 1); // Find ends of buckets
	for (i = n - 1, d += 1, b = SA + B[c1 = 0]; 0 <= i; --i) {
		if (0 < (j = SA[i])) {
			if (n <= j) {
				d += 1;
				j -= n;
			}
			assert(chr(j) <= chr(j + 1));
			if ((c0 = chr(j)) != c1) {
				B[c1] = b - SA;
				b = SA + B[c1 = c0];
			}
			assert((b - SA) <= i);
			--j;
			t = c0;
			t = (t << 1) | (chr(j) > c1);
			if (D[t] != d) {
				j += n;
				D[t] = d;
			}
			*--b = (t & 1) ? ~(j + 1) : j;
			SA[i] = 0;
		}
	}
}

static int postProcLMS2(int *SA, int n, int m) {
	int i, j, d, name;

	// Compact all the sorted LMS substrings into the first m items of SA.
	assert(0 < n);
	for (i = 0, name = 0; (j = SA[i]) < 0; ++i) {
		j = ~j;
		if (n <= j) {
			name += 1;
		}
		SA[i] = j;
		assert((i + 1) < n);
	}
	if (i < m) {
		for (d = i, ++i;; ++i) {
			assert(i < n);
			if ((j = SA[i]) < 0) {
				j = ~j;
				if (n <= j) {
					name += 1;
				}
				SA[d++] = j;
				SA[i] = 0;
				if (d == m) {
					break;
				}
			}
		}
	}
	if (name < m) {
		// Store the lexicographic names.
		for (i = m - 1, d = name + 1; 0 <= i; --i) {
			if (n <= (j = SA[i])) {
				j -= n;
				--d;
			}
			SA[m + (j >> 1)] = d;
		}
	} else {
		// Unset flags.
		for (i = 0; i < m; ++i) {
			if (n <= (j = SA[i])) {
				j -= n;
				SA[i] = j;
			}
		}
	}

	return name;
}

static void induceSA(const void *T, int *SA, int *C, int *B, int n, int k, int cs) {
	int *b, i, j;
	int c0, c1;

	// Compute SAl.
	if (C == B) {
		getCounts(T, C, n, k, cs);
	}
	getBuckets(C, B, k, 0); // Find starts of buckets
	j = n - 1;
	b = SA + B[c1 = chr(j)];
	*b++ = ((0 < j) && (chr(j - 1) < c1)) ? ~j : j;
	for (i = 0; i < n; ++i) {
		j = SA[i], SA[i] = ~j;
		if (0 < j) {
			--j;
			assert(chr(j) >= chr(j + 1));
			if ((c0 = chr(j)) != c1) {
				B[c1] = b - SA;
				b = SA + B[c1 = c0];
			}
			assert(i < (b - SA));
			*b++ = ((0 < j) && (chr(j - 1) < c1)) ? ~j : j;
		}
	}

	// Compute SAs.
	if (C == B) {
		getCounts(T, C, n, k, cs);
	}
	getBuckets(C, B, k, 1); // Find ends of buckets
	for (i = n - 1, b = SA + B[c1 = 0]; 0 <= i; --i) {
		if (0 < (j = SA[i])) {
			--j;
			assert(chr(j) <= chr(j + 1));
			if ((c0 = chr(j)) != c1) {
				B[c1] = b - SA;
				b = SA + B[c1 = c0];
			}
			assert((b - SA) <= i);
			*--b = ((j == 0) || (chr(j - 1) > c1)) ? ~j : j;
		} else {
			SA[i] = ~j;
		}
	}
}

int computeSA(const void *T, int *SA, int fs, int n, int k, int cs) {
	int *C, *B, *D, *RA, *b;
	int i, j, m, p, q, t, name, pidx = 0, newfs;
	int c0, c1;
	unsigned int flags;

	assert((T != NULL) && (SA != NULL));
	assert((0 <= fs) && (0 < n) && (1 <= k));

	if (k <= MINBUCKETSIZE) {
		if ((C = SAIS_MYMALLOC(k, int)) == NULL) {
			return -2;
		}
		if (k <= fs) {
			B = SA + (n + fs - k);
			flags = 1;
		} else {
			if ((B = SAIS_MYMALLOC(k, int)) == NULL) {
				SAIS_MYFREE(C, k, int);
				return -2;
			}
			flags = 3;
		}
	} else if (k <= fs) {
		C = SA + (n + fs - k);
		if (k <= (fs - k)) {
			B = C - k;
			flags = 0;
		} else if (k <= (MINBUCKETSIZE * 4)) {
			if ((B = SAIS_MYMALLOC(k, int)) == NULL) {
				return -2;
			}
			flags = 2;
		} else {
			B = C;
			flags = 8;
		}
	} else {
		if ((C = B = SAIS_MYMALLOC(k, int)) == NULL) {
			return -2;
		}
		flags = 4 | 8;
	}
	if ((n <= SAIS_LMSSORT2_LIMIT) && (2 <= (n / k))) {
		if (flags & 1) {
			flags |= ((k * 2) <= (fs - k)) ? 32 : 16;
		} else if ((flags == 0) && ((k * 2) <= (fs - k * 2))) {
			flags |= 32;
		}
	}

	// Stage 1: Reduce the problem by at least 1/2.
	// Sort all the LMS-substrings.
	getCounts(T, C, n, k, cs);
	getBuckets(C, B, k, 1); // Find ends of buckets
	for (i = 0; i < n; ++i) {
		SA[i] = 0;
	}
	b = &t;
	i = n - 1;
	j = n;
	m = 0;
	c0 = chr(n - 1);
	do {
		c1 = c0;
	}
	while ((0 <= --i) && ((c0 = chr(i)) >= c1));
	for (; 0 <= i;) {
		do {
			c1 = c0;
		}
		while ((0 <= --i) && ((c0 = chr(i)) <= c1));
		if (0 <= i) {
			*b = j;
			b = SA + --B[c1];
			j = i;
			++m;
			do {
				c1 = c0;
			}
			while ((0 <= --i) && ((c0 = chr(i)) >= c1));
		}
	}

	if (1 < m) {
		if (flags & (16 | 32)) {
			if (flags & 16) {
				if ((D = SAIS_MYMALLOC(k * 2, int)) == NULL) {
					if (flags & (1 | 4)) {
						SAIS_MYFREE(C, k, int);
					}
					if (flags & 2) {
						SAIS_MYFREE(B, k, int);
					}
					return -2;
				}
			} else {
				D = B - k * 2;
			}
			assert((j + 1) < n);
			++B[chr(j + 1)];
			for (i = 0, j = 0; i < k; ++i) {
				j += C[i];
				if (B[i] != j) {
					assert(SA[B[i]] != 0);
					SA[B[i]] += n;
				}
				D[i] = D[i + k] = 0;
			}
			LMSsort2(T, SA, C, B, D, n, k, cs);
			name = postProcLMS2(SA, n, m);
			if (flags & 16) {
				SAIS_MYFREE(D, k * 2, int);
			}
		} else {
			sortLMS1(T, SA, C, B, n, k, cs);
			name = postProcLMS1(T, SA, n, m, cs);
		}
	} else if (m == 1) {
		*b = j + 1;
		name = 1;
	} else {
		name = 0;
	}

	// Stage 2: Solve the reduced problem.
	// Recurse if names are not yet unique.
	if (name < m) {
		if (flags & 4) {
			SAIS_MYFREE(C, k, int);
		}
		if (flags & 2) {
			SAIS_MYFREE(B, k, int);
		}
		newfs = (n + fs) - (m * 2);
		if ((flags & (1 | 4 | 8)) == 0) {
			if ((k + name) <= newfs) {
				newfs -= k;
			} else {
				flags |= 8;
			}
		}
		assert((n >> 1) <= (newfs + m));
		RA = SA + m + newfs;
		for (i = m + (n >> 1) - 1, j = m - 1; m <= i; --i) {
			if (SA[i] != 0) {
				RA[j--] = SA[i] - 1;
			}
		}
		if (computeSA(RA, SA, newfs, m, name, sizeof(int)) != 0) {
			if (flags & 1) {
				SAIS_MYFREE(C, k, int);
			}
			return -2;
		}

		i = n - 1;
		j = m - 1;
		c0 = chr(n - 1);
		do {
			c1 = c0;
		}
		while ((0 <= --i) && ((c0 = chr(i)) >= c1));
		for (; 0 <= i;) {
			do {
				c1 = c0;
			}
			while ((0 <= --i) && ((c0 = chr(i)) <= c1));
			if (0 <= i) {
				RA[j--] = i + 1;
				do {
					c1 = c0;
				}
				while ((0 <= --i) && ((c0 = chr(i)) >= c1));
			}
		}
		for (i = 0; i < m; ++i) {
			SA[i] = RA[SA[i]];
		}
		if (flags & 4) {
			if ((C = B = SAIS_MYMALLOC(k, int)) == NULL) {
				return -2;
			}
		}
		if (flags & 2) {
			if ((B = SAIS_MYMALLOC(k, int)) == NULL) {
				if (flags & 1) {
					SAIS_MYFREE(C, k, int);
				}
				return -2;
			}
		}
	}

	// Stage 3: Induce the result for the original problem.
	if (flags & 8) {
		getCounts(T, C, n, k, cs);
	}
	// Put all left-most S characters into their buckets.
	if (1 < m) {
		getBuckets(C, B, k, 1); // Find ends of buckets
		i = m - 1, j = n, p = SA[m - 1], c1 = chr(p);
		do {
			q = B[c0 = c1];
			while (q < j) {
				SA[--j] = 0;
			}
			do {
				SA[--j] = p;
				if (--i < 0) {
					break;
				}
				p = SA[i];
			} while ((c1 = chr(p)) == c0);
		} while (0 <= i);
		while (0 < j) {
			SA[--j] = 0;
		}
	}
	induceSA(T, SA, C, B, n, k, cs);
	if (flags & (1 | 4)) {
		SAIS_MYFREE(C, k, int);
	}
	if (flags & 2) {
		SAIS_MYFREE(B, k, int);
	}
	return 0;
}
