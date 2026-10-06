package ast

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"testing"
)

func TestParseDecimal(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"", "+", "-", ".", "e5", "1e", "1e+", "1e5x", "1.2.3", "0x10", "1_0", "Inf", "NaN", "1e1234567890", "1234567890123456", "0.1234567890123456", "1.000000000000001"} {
		if _, ok := parseDecimal(s); ok {
			t.Errorf("expected %q not to parse", s)
		}
	}

	for _, s := range []string{"0", "-0", "0.0", "+5", "5.", ".5", "123456789012345", "1e-7", "1E+7", "1.500e2", "000123", "1.00000000000001"} {
		if _, ok := parseDecimal(s); !ok {
			t.Errorf("expected %q to parse", s)
		}
	}
}

// Whatever parses as a decimal is ordered exactly like big.Rat does, which is what the
// comparison of everything else falls back to.
func TestDecimalCompareMatchesExact(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(1))
	var pool []string
	for range 2000 {
		pool = append(pool, randomNumber(r))
	}
	// close to the numbers above, so that most pairs are not ordered by their magnitude
	for _, s := range pool[:1000] {
		pool = append(pool, s+"0", s+"1", "-"+strings.TrimPrefix(s, "-"), strings.Replace(s, ".", ".0", 1))
	}

	for range 200000 {
		x, y := pool[r.Intn(len(pool))], pool[r.Intn(len(pool))]
		xd, ok1 := parseDecimal(x)
		yd, ok2 := parseDecimal(y)
		if !ok1 || !ok2 {
			continue
		}
		rx, ok1 := new(big.Rat).SetString(x)
		ry, ok2 := new(big.Rat).SetString(y)
		if !ok1 || !ok2 {
			t.Fatalf("big.Rat cannot parse %q and %q", x, y)
		}
		if exp, got := rx.Cmp(ry), xd.compare(yd); exp != got {
			t.Fatalf("%s vs %s: expected %d but got %d", x, y, exp, got)
		}
	}
}

func randomNumber(r *rand.Rand) string {
	var sb strings.Builder
	if r.Intn(5) == 0 {
		sb.WriteByte('-')
	}
	digits := 1 + r.Intn(15)
	intDigits := r.Intn(digits + 1)
	digit := func() byte {
		if r.Intn(4) == 0 {
			return '0'
		}
		return byte('0' + r.Intn(10))
	}
	for range intDigits {
		sb.WriteByte(digit())
	}
	if intDigits == 0 {
		sb.WriteByte('0')
	}
	if intDigits < digits {
		sb.WriteByte('.')
		for range digits - intDigits {
			sb.WriteByte(digit())
		}
	}
	if r.Intn(4) == 0 {
		fmt.Fprintf(&sb, "e%+d", r.Intn(40)-20)
	}
	return sb.String()
}
