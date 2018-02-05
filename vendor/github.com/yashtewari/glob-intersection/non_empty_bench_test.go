package gintersect

import (
	"fmt"
	"testing"
)

// BenchmarkContinuousDotStarNonEmpty benchamarks two globs made solely of .*
func BenchmarkContinuousDotStarNonEmpty(b *testing.B) {
	lhs, rhs := "", ""
	dotStar := ".*"
	for i := 1; i <= 15; i++ {
		lhs = lhs + dotStar
		rhs = rhs + dotStar

		b.Run(fmt.Sprintf("with-%d-stars", i), func(b *testing.B) {
			_, err := NonEmpty(lhs, rhs)
			if err != nil {
				b.Error(err)
			}
		})
	}
}

// BenchmarkContinuousDotStarEmpty benchmarks two globs made solely of .*
// except for the mismatch at the end.
func BenchmarkContinuousDotStarEmpty(b *testing.B) {
	lhsPrefix, rhsPrefix := "", ""
	dotStar := ".*"
	for i := 1; i <= 15; i++ {
		lhsPrefix = lhsPrefix + dotStar
		rhsPrefix = rhsPrefix + dotStar

		lhs, rhs := lhsPrefix+"c", rhsPrefix+"d"

		b.Run(fmt.Sprintf("with-%d-stars", i), func(b *testing.B) {
			_, err := NonEmpty(lhs, rhs)
			if err != nil {
				b.Error(err)
			}
		})
	}
}

// The following benchmarks use glob strings having stars interspersed periodically.

// BenchmarkInterspersedStarsNonEmpty benchmarks two intersecting globs.
func BenchmarkInterspersedStarsNonEmpty(b *testing.B) {
	for i := 1; i <= 15; i++ {
		lhs, rhs := interspersedStars(i)

		b.Run(fmt.Sprintf("with-%d-stars", i), func(b *testing.B) {
			_, err := NonEmpty(lhs, rhs)
			if err != nil {
				b.Error(err)
			}
		})
	}
}

// BenchmarkInterspersedStarsEmpty1 benchmarks two non-intersecting globs
// such that the mismatch is in the middle of both globs.
func BenchmarkInterspersedStarsEmptyMiddle(b *testing.B) {
	for i := 1; i <= 15; i++ {
		// Construct lhs and rhs with the same intersecting prefixes and suffixes,
		// such that they have i stars each and a mismatching character in the middle.
		prefixL, prefixR := interspersedStars((i / 2) + (i % 2))
		suffixL, suffixR := interspersedStars(i / 2)
		lhs := prefixL + "c" + suffixL
		rhs := prefixR + "d" + suffixR

		b.Run(fmt.Sprintf("with-%d-stars", i), func(b *testing.B) {
			_, err := NonEmpty(lhs, rhs)
			if err != nil {
				b.Error(err)
			}
		})
	}
}

// BenchmarkInterspersedStarsEmpty1 benchmarks two non-intersecting globs
// such that the mismatch is near the end of both globs, but not at the very end.
// Various combinations of star counts are tried.
func BenchmarkInterspersedStarsEmptyEnd(b *testing.B) {
	// smallLimit specifies the maximum star count of the smaller of two inputs.
	// largeLimit specifies the maximum star count of the larger of two inputs.
	smallLimit, largeLimit := 10, 50
	inputL := make([]string, largeLimit+1, largeLimit+1)
	inputR := make([]string, largeLimit+1, largeLimit+1)

	for i := 1; i <= largeLimit; i++ {
		inputL[i], inputR[i] = interspersedStars(i - 1)
		inputL[i] += "c.*"
		inputR[i] += "d.*"
	}
	for i := 1; i <= largeLimit; i++ {
		for j := 1; j <= largeLimit; j++ {

			if i > smallLimit && j > smallLimit {
				continue
			}

			lhs, rhs := inputL[i], inputR[j]

			b.Run(fmt.Sprintf("with-%d-and-%d-stars", i, j), func(b *testing.B) {
				_, err := NonEmpty(lhs, rhs)
				if err != nil {
					b.Error(err)
				}
			})
		}
	}
}

// interspersedStars returns two intersecting, but not equal, glob strings s1 and s2 such that they each have count stars.
func interspersedStars(count int) (s1, s2 string) {
	star := "*"
	for r := 'a'; r < 'a'+rune(count); r++ {
		ch := string(r)
		// Add ch+ to s1
		s1 += ch + star
		// Add ch 5 times to both s1 and s2
		for i := 0; i < 5; i++ {
			s1 += ch
			s2 += ch
		}
		// Add ch* to s2
		s2 += ch + star
	}

	return
}
