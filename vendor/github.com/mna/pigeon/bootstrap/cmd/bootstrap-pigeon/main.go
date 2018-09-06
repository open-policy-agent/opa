// Command bootstrap-pigeon generates a PEG parser from a PEG grammar
// to bootstrap the pigeon command-line tool, as it is built using
// a simplified bootstrapping grammar that understands just enough of the
// pigeon grammar to parse it and build the tool.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"

	"golang.org/x/tools/imports"

	"github.com/mna/pigeon/ast"
	"github.com/mna/pigeon/builder"
)

func main() {
	dbgFlag := flag.Bool("debug", false, "set debug mode")
	noBuildFlag := flag.Bool("x", false, "do not build, only parse")
	outputFlag := flag.String("o", "", "output file, defaults to stdout")
	flag.Parse()

	if flag.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "USAGE: %s [options] [FILE]\n", os.Args[0])
		os.Exit(1)
	}

	nm := "stdin"
	inf := os.Stdin
	if flag.NArg() == 1 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer func() {
			err := f.Close()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(6)
			}
		}()
		inf = f
		nm = flag.Arg(0)
	}
	in := bufio.NewReader(inf)

	g, err := ParseReader(nm, in, Debug(*dbgFlag))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse error: ", err)
		os.Exit(3)
	}

	if !*noBuildFlag {
		outw := os.Stdout
		if *outputFlag != "" {
			f, err := os.Create(*outputFlag)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(4)
			}
			defer func() {
				err := f.Close()
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(7)
				}
			}()
			outw = f
		}

		outBuf := bytes.NewBuffer([]byte{})

		nolintOpt := builder.Nolint(true)
		if err := builder.BuildParser(outBuf, g.(*ast.Grammar), nolintOpt); err != nil {
			fmt.Fprintln(os.Stderr, "build error: ", err)
			os.Exit(5)
		}

		// Defaults from golang.org/x/tools/cmd/goimports
		options := &imports.Options{
			TabWidth:  8,
			TabIndent: true,
			Comments:  true,
			Fragment:  true,
		}

		formattedBuf, err := imports.Process("filename", outBuf.Bytes(), options)
		if err != nil {
			if _, err := outw.Write(outBuf.Bytes()); err != nil {
				fmt.Fprintln(os.Stderr, "write error: ", err)
				os.Exit(7)
			}
			fmt.Fprintln(os.Stderr, "format error: ", err)
			os.Exit(6)
		}

		if _, err := outw.Write(formattedBuf); err != nil {
			fmt.Fprintln(os.Stderr, "write error: ", err)
			os.Exit(7)
		}
	}
}

func (c *current) astPos() ast.Pos {
	return ast.Pos{Line: c.pos.line, Col: c.pos.col, Off: c.pos.offset}
}

func toIfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	return v.([]interface{})
}
