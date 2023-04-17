// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/test/cases"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

type params struct {
	Output           string
	InputDir         string
	TestFilePatterns []string
	TestRunner       string
}

type compiledTestCaseSet struct {
	Cases []compiledTestCase `json:"cases"`
}

type compiledTestCase struct {
	cases.TestCase
	WASM []byte `json:"wasm,omitempty"`
}

func compileTestCases(ctx context.Context, tests cases.Set) (*compiledTestCaseSet, error) {
	result := make([]compiledTestCase, 0, len(tests.Cases))
	for _, tc := range tests.Cases {

		var numExpects int

		if tc.WantDefined != nil {
			numExpects++
		}

		if tc.WantError != nil {
			numExpects++
		}

		if tc.WantResult != nil {
			numExpects++
		}

		if numExpects != 1 {
			return nil, fmt.Errorf("test case %v: must specify exactly one expectation (e.g., want_defined) but got %v", tc.Note, numExpects)
		}

		args := []func(*rego.Rego){
			rego.Query(tc.Query),
			rego.FunctionDecl(&rego.Function{
				Name: "custom_builtin_test",
				Decl: types.NewFunction(
					[]types.Type{types.N},
					types.N,
				),
			}),
			rego.FunctionDecl(&rego.Function{
				Name: "custom_builtin_test_impure",
				Decl: types.NewFunction(
					[]types.Type{},
					types.N,
				),
			}),
			rego.FunctionDecl(&rego.Function{
				Name: "custom_builtin_test_memoization",
				Decl: types.NewFunction(
					[]types.Type{},
					types.N,
				),
			}),
		}

		for idx, module := range tc.Modules {
			args = append(args, rego.Module(fmt.Sprintf("module%d.rego", idx), module))
		}

		var bs []byte

		cr, err := rego.New(args...).Compile(ctx)
		if err != nil {
			return nil, err
		}

		bs = cr.Bytes

		result = append(result, compiledTestCase{
			TestCase: tc,
			WASM:     bs,
		})
	}

	return &compiledTestCaseSet{Cases: result}, nil
}

func pathMatchesAny(patterns []string, p string) (bool, error) {
	for i := range patterns {
		if ok, err := path.Match(patterns[i], p); err != nil {
			return false, err
		} else if ok {
			return true, nil
		}
	}
	return false, nil
}

func run(params params) error {

	ctx := context.Background()

	if err := os.MkdirAll(path.Dir(params.Output), 0755); err != nil {
		return err
	}

	f, err := os.Create(params.Output)
	if err != nil {
		return err
	}

	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	files, err := os.ReadDir(params.InputDir)
	if err != nil {
		return err
	}

	for i := range files {
		if ok, err := pathMatchesAny(params.TestFilePatterns, files[i].Name()); ok {
			err := func() error {
				abspath := filepath.Join(params.InputDir, files[i].Name())

				bs, err := os.ReadFile(abspath)
				if err != nil {
					return err
				}

				var tcs cases.Set

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
				return writeFile(tw, dst, bs)
			}()
			if err != nil {
				return fmt.Errorf("%s: %w", files[i].Name(), err)
			}
		} else if err != nil {
			return fmt.Errorf("%s: %w", files[i].Name(), err)
		}
	}

	return copyFile(tw, "test.js", params.TestRunner)
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
			return run(params)
		},
	}

	command.Flags().StringVarP(&params.Output, "output", "", "", "set path of output file")
	command.Flags().StringVarP(&params.InputDir, "input-dir", "", "", "set path of input directory containing test files")
	command.Flags().StringSliceVarP(&params.TestFilePatterns, "file-pattern", "", []string{"*.yaml", "*.json"}, "set filename patterns to match test files against")
	command.Flags().StringVarP(&params.TestRunner, "runner", "", "", "set path of test runner")

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
