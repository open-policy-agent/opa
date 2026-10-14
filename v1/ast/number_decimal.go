// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "math/bits"

// maxDecimalDigits is the number of significant digits up to which a number is
// held as a decimal. Any two different decimals of this many digits or fewer
// are different float64s too (float64 holds 15 digits), so comparing them
// exactly agrees with how NumberCompare has always treated floats.
const maxDecimalDigits = 15

// maxDecimalExp bounds the position of a decimal's leading digit. Beyond it, big.Float may
// underflow or overflow, which NumberCompare has to keep treating as it always has.
const maxDecimalExp = 300

var pow10 = [...]uint64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9,
	1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}

// decimal is the exact value of a number: (-1)^neg * m * 10^e.
type decimal struct {
	m   uint64
	e   int
	neg bool
}

// parseDecimal parses s without allocating. It reports false for anything that is not a plain
// decimal or exponent form with at most maxDecimalDigits significant digits.
func parseDecimal(s string) (d decimal, ok bool) {
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		d.neg = s[i] == '-'
		i++
	}

	var digits int
	var seenDigit, seenDot bool
	for ; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			seenDigit = true
			if d.m != 0 || c != '0' {
				digits++
				if digits > maxDecimalDigits {
					return decimal{}, false
				}
				d.m = d.m*10 + uint64(c-'0')
			}
			if seenDot {
				d.e--
			}
			continue
		}
		if c == '.' && !seenDot {
			seenDot = true
			continue
		}
		break
	}
	if !seenDigit {
		return decimal{}, false
	}

	if i < len(s) {
		if s[i] != 'e' && s[i] != 'E' {
			return decimal{}, false
		}
		i++
		expNeg := false
		if i < len(s) && (s[i] == '-' || s[i] == '+') {
			expNeg = s[i] == '-'
			i++
		}
		if i == len(s) || len(s)-i > 9 {
			return decimal{}, false
		}
		var exp int
		for ; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return decimal{}, false
			}
			exp = exp*10 + int(s[i]-'0')
		}
		if expNeg {
			exp = -exp
		}
		d.e += exp
	}

	if d.m == 0 {
		return decimal{}, true
	}
	if p := decimalDigits(d.m) + d.e; p > maxDecimalExp || p < -maxDecimalExp {
		return decimal{}, false
	}
	return d, true
}

// compare returns -1, 0 or 1, depending on how d orders against o.
func (d decimal) compare(o decimal) int {
	switch {
	case d.m == 0 && o.m == 0:
		return 0
	case d.m == 0:
		return sign(!o.neg)
	case o.m == 0:
		return sign(d.neg)
	case d.neg != o.neg:
		return sign(d.neg)
	}

	c := d.compareMagnitude(o)
	if d.neg {
		return -c
	}
	return c
}

func sign(negative bool) int {
	if negative {
		return -1
	}
	return 1
}

func (d decimal) compareMagnitude(o decimal) int {
	// Position of the leading digit. With equal positions, the numbers align by scaling the
	// one with the larger exponent, which has as many digits as the other.
	dp, op := decimalDigits(d.m)+d.e, decimalDigits(o.m)+o.e
	switch {
	case dp < op:
		return -1
	case dp > op:
		return 1
	}

	dm, om := d.m, o.m
	if d.e > o.e {
		dm *= pow10[d.e-o.e]
	} else {
		om *= pow10[o.e-d.e]
	}

	switch {
	case dm < om:
		return -1
	case dm > om:
		return 1
	}
	return 0
}

// decimalDigits returns the number of decimal digits in m, which must not be zero.
func decimalDigits(m uint64) int {
	n := (bits.Len64(m) * 1233) >> 12
	if m >= pow10[n] {
		n++
	}
	return n
}
