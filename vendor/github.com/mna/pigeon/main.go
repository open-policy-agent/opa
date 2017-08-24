package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/tools/imports"

	"github.com/mna/pigeon/ast"
	"github.com/mna/pigeon/builder"
)

var exit = os.Exit

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// define command-line flags
	var (
		cacheFlag              = fs.Bool("cache", false, "cache parsing results")
		dbgFlag                = fs.Bool("debug", false, "set debug mode")
		shortHelpFlag          = fs.Bool("h", false, "show help page")
		longHelpFlag           = fs.Bool("help", false, "show help page")
		noRecoverFlag          = fs.Bool("no-recover", false, "do not recover from panic")
		outputFlag             = fs.String("o", "", "output file, defaults to stdout")
		optimizeBasicLatinFlag = fs.Bool("optimize-basic-latin", false, "generate optimized parser for Unicode Basic Latin character sets")
		optimizeGrammar        = fs.Bool("optimize-grammar", false, "optimize the given grammar (EXPERIMENTAL FEATURE)")
		optimizeParserFlag     = fs.Bool("optimize-parser", false, "generate optimized parser without Debug and Memoize options")
		recvrNmFlag            = fs.String("receiver-name", "c", "receiver name for the generated methods")
		noBuildFlag            = fs.Bool("x", false, "do not build, only parse")
	)

	fs.Usage = usage
	err := fs.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "args parse error:\n", err)
		exit(6)
	}

	if *shortHelpFlag || *longHelpFlag {
		fs.Usage()
		exit(0)
	}

	if fs.NArg() > 1 {
		argError(1, "expected one argument, got %q", strings.Join(fs.Args(), " "))
	}

	// get input source
	infile := ""
	if fs.NArg() == 1 {
		infile = fs.Arg(0)
	}
	nm, rc := input(infile)
	defer func() {
		err = rc.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "close file error:\n", err)
		}
		if r := recover(); r != nil {
			panic(r)
		}
		if err != nil {
			exit(7)
		}
	}()

	// parse input
	g, err := ParseReader(nm, rc, Debug(*dbgFlag), Memoize(*cacheFlag), Recover(!*noRecoverFlag))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse error(s):\n", err)
		exit(3)
	}

	if !*noBuildFlag {
		if *optimizeGrammar {
			ast.Optimize(g.(*ast.Grammar))
		}

		// generate parser
		out := output(*outputFlag)
		defer func() {
			err := out.Close()
			if err != nil {
				fmt.Fprintln(os.Stderr, "close file error:\n", err)
				exit(8)
			}
		}()

		outBuf := bytes.NewBuffer([]byte{})

		curNmOpt := builder.ReceiverName(*recvrNmFlag)
		optimizeParser := builder.Optimize(*optimizeParserFlag)
		basicLatinOptimize := builder.BasicLatinLookupTable(*optimizeBasicLatinFlag)
		if err := builder.BuildParser(outBuf, g.(*ast.Grammar), curNmOpt, optimizeParser, basicLatinOptimize); err != nil {
			fmt.Fprintln(os.Stderr, "build error: ", err)
			exit(5)
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
			if _, err := out.Write(outBuf.Bytes()); err != nil {
				fmt.Fprintln(os.Stderr, "write error: ", err)
				exit(7)
			}
			fmt.Fprintln(os.Stderr, "format error: ", err)
			exit(6)
		}

		if _, err := out.Write(formattedBuf); err != nil {
			fmt.Fprintln(os.Stderr, "write error: ", err)
			exit(7)
		}
	}
}

var usagePage = `usage: %s [options] [GRAMMAR_FILE]

Pigeon generates a parser based on a PEG grammar.

By default, pigeon reads the grammar from stdin and writes the
generated parser to stdout. If GRAMMAR_FILE is specified, the
grammar is read from this file instead. If the -o flag is set,
the generated code is written to this file instead.

	-cache
		cache parser results to avoid exponential parsing time in
		pathological cases. Can make the parsing slower for typical
		cases and uses more memory.
	-debug
		output debugging information while parsing the grammar.
	-h -help
		display this help message.
	-no-recover
		do not recover from a panic. Useful to access the panic stack
		when debugging, otherwise the panic is converted to an error.
	-o OUTPUT_FILE
		write the generated parser to OUTPUT_FILE. Defaults to stdout.
	-optimize-basic-latin
		generate optimized parser for Unicode Basic Latin character set
	-optimize-grammar
		performes several performance optimizations on the grammar (EXPERIMENTAL FEATURE)
	-optimize-parser
		generate optimized parser without Debug and Memoize options
	-receiver-name NAME
		use NAME as for the receiver name of the generated methods
		for the grammar's code blocks. Defaults to "c".
	-x
		do not generate the parser, only parse the grammar.

See https://godoc.org/github.com/mna/pigeon for more information.
`

// usage prints the help page of the command-line tool.
func usage() {
	fmt.Printf(usagePage, os.Args[0])
}

// argError prints an error message to stderr, prints the command usage
// and exits with the specified exit code.
func argError(exitCode int, msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintln(os.Stderr)
	usage()
	exit(exitCode)
}

// input gets the name and reader to get input text from.
func input(filename string) (nm string, rc io.ReadCloser) {
	nm = "stdin"
	inf := os.Stdin
	if filename != "" {
		f, err := os.Open(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exit(2)
		}
		inf = f
		nm = filename
	}
	r := bufio.NewReader(inf)
	return nm, makeReadCloser(r, inf)
}

// output gets the writer to write the generated parser to.
func output(filename string) io.WriteCloser {
	out := os.Stdout
	if filename != "" {
		f, err := os.Create(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exit(4)
		}
		out = f
	}
	return out
}

// create a ReadCloser that reads from r and closes c.
func makeReadCloser(r io.Reader, c io.Closer) io.ReadCloser {
	rc := struct {
		io.Reader
		io.Closer
	}{r, c}
	return io.ReadCloser(rc)
}

// astPos is a helper method for the PEG grammar parser. It returns the
// position of the current match as an ast.Pos.
func (c *current) astPos() ast.Pos {
	return ast.Pos{Line: c.pos.line, Col: c.pos.col, Off: c.pos.offset}
}

// toIfaceSlice is a helper function for the PEG grammar parser. It converts
// v to a slice of empty interfaces.
func toIfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	return v.([]interface{})
}

// validateUnicodeEscape checks that the provided escape sequence is a
// valid Unicode escape sequence.
func validateUnicodeEscape(escape, errMsg string) (interface{}, error) {
	r, _, _, err := strconv.UnquoteChar("\\"+escape, '"')
	if err != nil {
		return nil, errors.New(errMsg)
	}
	if 0xD800 <= r && r <= 0xDFFF {
		return nil, errors.New(errMsg)
	}
	return nil, nil
}
