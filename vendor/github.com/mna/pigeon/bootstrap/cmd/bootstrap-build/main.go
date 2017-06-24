// Command bootstrap-build bootstraps the PEG parser generator by
// parsing the bootstrap grammar and creating a basic parser generator
// sufficiently complete to parse the pigeon PEG grammar.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/tools/imports"

	"github.com/mna/pigeon/bootstrap"
	"github.com/mna/pigeon/builder"
)

func main() {
	outFlag := flag.String("o", "", "output file, defaults to stdout")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "USAGE: bootstrap-build [-o OUTPUT] FILE")
		os.Exit(1)
	}

	outw := os.Stdout
	if *outFlag != "" {
		outf, err := os.Create(*outFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer func() {
			err := outf.Close()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(3)
			}
		}()
		outw = outf
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer func() {
		err = f.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	}()

	p := bootstrap.NewParser()
	g, err := p.Parse(os.Args[1], f)
	if err != nil {
		log.Fatal(err)
	}

	outBuf := bytes.NewBuffer([]byte{})

	if err := builder.BuildParser(outBuf, g); err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}
		log.Fatal("format error" + err.Error())
	}

	if _, err := outw.Write(formattedBuf); err != nil {
		log.Fatal(err)
	}
}
