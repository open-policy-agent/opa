// Package bootstrap implements the scanner and parser to bootstrap the
// PEG parser generator.
//
// It parses the PEG grammar into an ast that is then used to generate
// a parser generator based on this PEG grammar. The generated parser
// can then parse the grammar again, without the bootstrap package.
package bootstrap
