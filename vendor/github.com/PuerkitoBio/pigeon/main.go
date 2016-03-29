package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/pigeon/ast"
	"github.com/PuerkitoBio/pigeon/builder"
)

var exit = os.Exit

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// define command-line flags
	var (
		cacheFlag     = fs.Bool("cache", false, "cache parsing results")
		dbgFlag       = fs.Bool("debug", false, "set debug mode")
		shortHelpFlag = fs.Bool("h", false, "show help page")
		longHelpFlag  = fs.Bool("help", false, "show help page")
		noRecoverFlag = fs.Bool("no-recover", false, "do not recover from panic")
		outputFlag    = fs.String("o", "", "output file, defaults to stdout")
		recvrNmFlag   = fs.String("receiver-name", "c", "receiver name for the generated methods")
		noBuildFlag   = fs.Bool("x", false, "do not build, only parse")
	)

	fs.Usage = usage
	fs.Parse(os.Args[1:])

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
	defer rc.Close()

	// parse input
	g, err := ParseReader(nm, rc, Debug(*dbgFlag), Memoize(*cacheFlag), Recover(!*noRecoverFlag))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse error(s):\n", err)
		exit(3)
	}

	if !*noBuildFlag {
		// generate parser
		out := output(*outputFlag)
		defer out.Close()

		curNmOpt := builder.ReceiverName(*recvrNmFlag)
		if err := builder.BuildParser(out, g.(*ast.Grammar), curNmOpt); err != nil {
			fmt.Fprintln(os.Stderr, "build error: ", err)
			exit(5)
		}
	}
}

var usagePage = `usage: %s [options] [GRAMMAR_FILE]

Pigeon generates a parser based on a PEG grammar. It doesn't try
to format the generated code nor to detect required imports -
it is recommended to pipe the output of pigeon through a tool
such as goimports to do this, e.g.:

	pigeon GRAMMAR_FILE | goimports > output.go

Use the following command to install goimports:

	go get golang.org/x/tools/cmd/goimports

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
	-receiver-name NAME
		use NAME as for the receiver name of the generated methods
		for the grammar's code blocks. Defaults to "c".
	-x
		do not generate the parser, only parse the grammar.

See https://godoc.org/github.com/PuerkitoBio/pigeon for more
information.
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
