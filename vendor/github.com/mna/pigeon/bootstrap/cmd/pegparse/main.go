// Command pegparse is a helper command-line tool to test the bootstrap
// parser.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/mna/pigeon/bootstrap"
)

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "USAGE: pegparse FILE")
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

	p := bootstrap.NewParser()
	if _, err := p.Parse(nm, in); err != nil {
		log.Fatal(err)
	}
}
