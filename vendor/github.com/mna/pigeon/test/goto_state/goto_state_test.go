package asmgotostate

import (
	"strings"
	"testing"
)

func TestGoto(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		err      string
	}{
		{
			input:    `noop`,
			expected: `noop`,
			err:      ``,
		},
		{
			input:    `unused: noop`,
			expected: `noop`,
			err:      ``,
		},
		{
			input:    `self: jump self`,
			expected: `jump 0`,
			err:      ``,
		},
		{
			input: `prev: noop
jump prev`,
			expected: `noop
jump -1`,
			err: ``,
		},
		{
			input: `jump next
next: noop`,
			expected: `jump 1
noop`,
			err: ``,
		},
		{
			input: `start: noop
jump start
jump start
jump start
noop`,
			expected: `noop
jump -1
jump -2
jump -3
noop`,
			err: ``,
		},
		{
			input: `jump end
jump end
jump end
end: jump end`,
			expected: `jump 3
jump 2
jump 1
jump 0`,
			err: ``,
		},
		{
			input: `noop
jump l1
l3: noop
jump l2
noop
l1: jump l3
l2: noop`,
			expected: `noop
jump 4
noop
jump 3
noop
jump -3
noop`,
			err: ``,
		},
		{
			input:    `jump undefined`,
			expected: ``,
			err:      `jump to undefined label 'undefined' found`,
		},
		{
			input: `multidefined: noop
multidefined: noop`,
			expected: ``,
			err:      `label 'multidefined' already defined on line 1`,
		},
	}

	for _, test := range cases {
		ll := make(labelLookup)
		got, err := Parse("", []byte(test.input), InitState("labelLookup", ll))
		if err != nil {
			if !strings.Contains(err.Error(), test.err) {
				t.Fatalf("Parse error did not match, expected: %s, got %s, input:\n%s\n", test.err, err, test.input)
			}
		} else {
			instr := got.([]Instruction)
			result := ""
			for _, inst := range instr {
				result += inst.Assemble() + "\n"
			}
			result = strings.Trim(result, "\n")
			if result != test.expected {
				t.Fatalf("Parse did not provide expected result, expected:\n%s\n\nOutput:\n%s\n", test.expected, result)
			}
		}
	}
}
