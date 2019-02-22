package gintersect

import (
	"testing"
)

var (
	matching, nonMatching map[Token][]Token
)

func init() {
	initializeTestSamples()
}

func TestMatching(t *testing.T) {
	tests := map[Token][]Token{
		testCharacters['a']: []Token{testCharacters['a'], testLowerAlphaSet, testLowerAlphaSetPlus, testDot},
		testCharacters['P']: []Token{testUpperAlphaSetStar, testDot},
		testDotPlus:         []Token{testDotStar, testSymbolSet, testNumSetPlus},
		testSymbolSet:       []Token{testCharacters['.'], testCharacters['+'], NewSet([]rune{'.', 'x'})},
		testNumSetPlus:      []Token{testCharacters['0'], testCharacters['9'], testDotStar, NewSet([]rune{'~', 'T', '4'})},
	}

	for t1, t2s := range tests {
		for _, t2 := range t2s {
			if !Match(t1, t2) {
				t.Errorf("expected %s and %s to match, but they didn't", t1.String(), t2.String())
			}
		}
	}
}

func TestNonMatching(t *testing.T) {
	tests := map[Token][]Token{
		testCharacters['d']: []Token{testCharacters['D'], testCharacters['b'], testNumSet},
		testNumSetPlus:      []Token{testCharacters['.'], testCharacters['g'], testSymbolSetPlus, testLowerAlphaSet},
		testUpperAlphaSet:   []Token{testCharacters['5'], testCharacters['j'], testSymbolSetStar, testLowerAlphaSetPlus},
	}

	for t1, t2s := range tests {
		for _, t2 := range t2s {
			if Match(t1, t2) {
				t.Errorf("expected %s and %s not to match, but they did", t1.String(), t2.String())
			}
		}
	}
}
