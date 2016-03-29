package main

import (
	"os"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, _ = os.Open(os.DevNull)
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() {
		exit = os.Exit
		os.Stdout = stdout
		os.Stderr = stderr
	}()
	exit = func(code int) {
		panic(code)
	}

	cases := []struct {
		args string
		code int
	}{
		{args: "", code: 3},            // stdin: no match found
		{args: "-h", code: 0},          // help
		{args: "FILE1 FILE2", code: 1}, // want only 1 non-flag arg
		{args: "-x", code: 3},          // stdin: no match found
	}

	for _, tc := range cases {
		os.Args = append([]string{"pigeon"}, strings.Fields(tc.args)...)

		got := runMainRecover()
		if got != tc.code {
			t.Errorf("%q: want code %d, got %d", tc.args, tc.code, got)
		}
	}
}

func runMainRecover() (code int) {
	defer func() {
		if e := recover(); e != nil {
			if i, ok := e.(int); ok {
				code = i
				return
			}
			panic(e)
		}
	}()
	main()
	return 0
}
