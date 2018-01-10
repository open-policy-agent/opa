package main

import (
	"path/filepath"
	"testing"
)

var cases = map[string]int{
	"significant_whitespace.txt": 42,
}

func TestIndentation(t *testing.T) {
	const rootDir = "testdata"
	for fname, exp := range cases {
		file := filepath.Join(rootDir, fname)
		pgot, err := ParseFile(file)
		if err != nil {
			t.Errorf("%s: pigeon.ParseFile: %v", file, err)
			continue
		}
		got, err := pgot.(ProgramNode).exec()
		if err != nil {
			t.Errorf("%s: ProgramNode.exec: %v", file, err)
			continue
		}

		if got != exp {
			t.Errorf("%v: want %v, got %v", file, exp, got)
		}
	}
}
