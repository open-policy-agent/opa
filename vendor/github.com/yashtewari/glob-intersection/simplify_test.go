package gintersect

import (
	"testing"
)

func init() {
	initializeTestSamples()
}

func TestSimplify(t *testing.T) {
	tests := []struct {
		input   []Token
		desired []Token
	}{
		{
			[]Token{testCharacters['a'], testCharacters['b'], testCharacters['c'], testCharacters['d']},
			[]Token{testCharacters['a'], testCharacters['b'], testCharacters['c'], testCharacters['d']},
		},
		{
			[]Token{testCharactersStar['p'], testCharactersStar['p'], testCharactersStar['p']},
			[]Token{testCharactersStar['p']},
		},
		{
			[]Token{testCharactersPlus['x'], testCharactersStar['x']},
			[]Token{testCharactersPlus['x']},
		},
		{
			[]Token{testNumSetPlus, testNumSetPlus, testNumSetStar, testSymbolSet, testSymbolSetStar, testSymbolSetPlus, testCharacters['4']},
			[]Token{testNumSetPlus, testSymbolSet, testSymbolSetPlus, testCharacters['4']},
		},
		{
			[]Token{testDotStar, testDotPlus, testDotStar, testDotPlus, testDotStar, testDotPlus, testDotStar, testDotPlus},
			[]Token{testDotPlus},
		},
	}

	for _, test := range tests {
		actual := Simplify(test.input)
		if !tokensEqual(test.desired, actual) {
			t.Errorf("simplifying: %s, desired: %s, actual %s", tokensString(test.input), tokensString(test.desired), tokensString(actual))
		}
	}
}
