// Command bootstrap-build bootstraps the PEG parser generator by
// parsing the bootstrap grammar and creating a basic parser generator
// sufficiently complete to parse the pigeon PEG grammar.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/PuerkitoBio/pigeon/bootstrap"
	"github.com/PuerkitoBio/pigeon/builder"
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
		defer outf.Close()
		outw = outf
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer f.Close()

	p := bootstrap.NewParser()
	g, err := p.Parse(os.Args[1], f)
	if err != nil {
		log.Fatal(err)
	}

	if err := builder.BuildParser(outw, g); err != nil {
		log.Fatal(err)
	}
}
