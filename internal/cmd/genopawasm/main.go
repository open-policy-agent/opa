// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/spf13/cobra"
)

type params struct {
	Output string
}

func main() {

	var params params
	executable := path.Base(os.Args[0])

	command := &cobra.Command{
		Use:   executable,
		Short: executable + " <opa.wasm path>",
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("provide path of opa.wasm file")
			}
			return run(params, args)
		},
	}

	command.Flags().StringVarP(&params.Output, "output", "o", "", "set path of output file (default: stdout)")

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(params params, args []string) error {

	var out io.Writer

	if params.Output != "" {
		f, err := os.Create(params.Output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	} else {
		out = os.Stdout
	}

	_, err := out.Write([]byte(`// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// THIS FILE IS GENERATED. DO NOT EDIT.

// Package opa contains bytecode for the OPA-WASM library.
package opa

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

// Bytes returns the OPA-WASM bytecode.
func Bytes() ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewBuffer(gzipped))
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(gr)
}

`))

	if err != nil {
		return err
	}

	_, err = out.Write([]byte(`var gzipped = []byte("`))
	if err != nil {
		return err
	}

	in, err := os.Open(args[0])
	if err != nil {
		return err
	}

	defer in.Close()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err = io.Copy(gw, in)
	if err != nil {
		return err
	}

	gw.Close()

	for _, b := range buf.Bytes() {
		if _, err := out.Write([]byte(`\x`)); err != nil {
			return err
		}
		if _, err := out.Write(asciihex(b)); err != nil {
			return err
		}
	}

	_, err = out.Write([]byte(`")`))
	if err != nil {
		return err
	}

	_, err = out.Write([]byte("\n"))
	return err
}

var digits = "0123456789ABCDEF"

func asciihex(b byte) []byte {
	lo := digits[(b & 0x0F)]
	hi := digits[(b >> 4)]
	return []byte{hi, lo}
}
