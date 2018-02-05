package gintersect

import (
	"fmt"
	"strings"
	"testing"
)

var (
	validInputs   map[string][]Token
	invalidInputs []string
)

func init() {
	initializeTestSamples()

	validInputs = map[string][]Token{
		"abcd":        []Token{testCharacters['a'], testCharacters['b'], testCharacters['c'], testCharacters['d']},
		"ab+cd+":      []Token{testCharacters['a'], testCharactersPlus['b'], testCharacters['c'], testCharactersPlus['d']},
		"a*b":         []Token{testCharactersStar['a'], testCharacters['b']},
		"a\\*b":       []Token{testCharacters['a'], testCharacters['*'], testCharacters['b']},
		"a.c.":        []Token{testCharacters['a'], testDot, testCharacters['c'], testDot},
		".*x*y*":      []Token{testDotStar, testCharactersStar['x'], testCharactersStar['y']},
		"\\.\\.\\.+":  []Token{testCharacters['.'], testCharacters['.'], testCharactersPlus['.']},
		"[a-z]+":      []Token{testLowerAlphaSetPlus},
		"[0-9]\\*":    []Token{testNumSet, testCharacters['*']},
		"[A-Z]*[a-z]": []Token{testUpperAlphaSetStar, testLowerAlphaSet},
		"[][][][]":    []Token{testEmptySet, testEmptySet, testEmptySet, testEmptySet},
	}

	invalidInputs = []string{
		"\\",
		"+",
		"abcd\\",
		"\\[]",
		"abcd[asdjfl",
		"abcd[",
		"abcd]",
		"abcd]asdf",
		"[120-9-4]+",
		"[a-z]++",
		"[][",
		"][[",
		"pq[a-]",
		"[a-z",
		"[123a-z-]",
		"\\.**",
		"[z-a]",
	}
}

func TestTokenizerValid(t *testing.T) {
	for input, desired := range validInputs {

		actual, err := Tokenize([]rune(input))
		if err != nil {
			t.Error(err)
		}

		if !tokensEqual(desired, actual) {
			t.Fatalf("incorrectly tokenized input: %s, wanted: %v, got: %v", input, tokensString(desired), tokensString(actual))
		}
	}
}

func TestTokenizerInvalid(t *testing.T) {
	for _, input := range invalidInputs {
		output, err := Tokenize([]rune(input))
		if err == nil {
			t.Errorf("expected error for input: %s, instead got output: %v", input, tokensString(output))
		}
	}
}

func tokensEqual(t1, t2 []Token) bool {
	if len(t1) != len(t2) {
		return false
	}
	for i, t := range t1 {
		if !t.Equal(t2[i]) {
			return false
		}
	}

	return true
}

func tokensString(tokens []Token) string {
	ts := make([]string, 0, 30)
	for _, t := range tokens {
		ts = append(ts, t.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(ts, ", "))
}
