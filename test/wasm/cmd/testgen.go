// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type params struct {
	Output   string
	InputDir string
}

type testCaseSet struct {
	Cases []testCase `json:"cases"`
}

type testCase struct {
	Note       string       `json:"note"`
	Query      string       `json:"query"`
	Input      *interface{} `json:"input"`
	ReturnCode int          `json:"return_code"`
}

type compiledTestCaseSet struct {
	Cases []compiledTestCase `json:"cases"`
}

type compiledTestCase struct {
	testCase
	WASM []byte `json:"wasm"`
}

func compileTestCases(ctx context.Context, tests testCaseSet) (*compiledTestCaseSet, error) {
	var result []compiledTestCase
	for _, tc := range tests.Cases {
		cr, err := rego.New(rego.Query(tc.Query)).Compile(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, compiledTestCase{
			testCase: tc,
			WASM:     cr.Bytes,
		})
	}
	return &compiledTestCaseSet{Cases: result}, nil
}

func run(params params, args []string) error {

	ctx := context.Background()

	f, err := os.Create(params.Output)
	if err != nil {
		return err
	}

	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	files, err := ioutil.ReadDir(params.InputDir)
	if err != nil {
		return err
	}

	for i := range files {
		if strings.HasSuffix(files[i].Name(), ".yaml") {

			err := func() error {
				bs, err := ioutil.ReadFile(filepath.Join(params.InputDir, files[i].Name()))
				if err != nil {
					return err
				}

				var tcs testCaseSet
				if err := util.Unmarshal(bs, &tcs); err != nil {
					return err
				}

				ctcs, err := compileTestCases(ctx, tcs)
				if err != nil {
					return err
				}

				bs, err = json.Marshal(ctcs)
				if err != nil {
					return err
				}

				dst := strings.Replace(files[i].Name(), ".yaml", ".json", -1)
				if err := writeFile(tw, dst, bs); err != nil {
					return err
				}
				return nil
			}()

			if err != nil {
				return errors.Wrap(err, files[i].Name())
			}
		}
	}

	return copyFile(tw, "test.js", filepath.Join(params.InputDir, "test.js"))
}

func writeFile(tw *tar.Writer, dst string, bs []byte) error {
	hdr := &tar.Header{
		Name:     strings.TrimLeft(dst, "/"),
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(bs)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	_, err := tw.Write(bs)
	return err
}

func copyFile(tw *tar.Writer, dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer in.Close()

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:     strings.TrimLeft(dst, "/"),
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     info.Size(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	_, err = io.Copy(tw, in)
	return err

}

func main() {

	var params params
	executable := path.Base(os.Args[0])

	command := &cobra.Command{
		Use:   executable,
		Short: executable,
		RunE: func(_ *cobra.Command, args []string) error {
			return run(params, args)
		},
	}

	command.Flags().StringVarP(&params.Output, "output", "o", "", "set path of output file")
	command.Flags().StringVarP(&params.InputDir, "input-dir", "i", "", "set path of input directory containing test files")

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
