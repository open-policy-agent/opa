// Command pegscan is a helper command-line tool to test the bootstrap
// scanner.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/mna/pigeon/bootstrap"
)

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "USAGE: pegscan FILE")
		os.Exit(1)
	}

	var in io.Reader

	nm := "stdin"
	if len(os.Args) == 2 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer func() {
			err := f.Close()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(3)
			}
		}()
		in = f
		nm = os.Args[1]
	} else {
		in = bufio.NewReader(os.Stdin)
	}

	var s bootstrap.Scanner
	s.Init(nm, in, nil)
	for {
		tok, ok := s.Scan()
		fmt.Println(tok)
		if !ok {
			break
		}
	}
}
