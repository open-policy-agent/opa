// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/util/test"
)

func TestWriteTokenToFile(t *testing.T) {

	token := `eyJhbGciOiJSUzI1NiJ9.eyJmaWxlcyI6W3sibmFtZSI6ImJ1bmRsZS8ubWFuaWZlc3QiLCJoYXNoIjoiZWUwZWRiZGZkMjgzNTBjNDk2ZjA4ODI3Y2E1Y2VhYjgwMzA2NzI0YjYyZGY1ZjY0MDRlNzBjYjc2NjYxNWQ5ZCIsImFsZ29yaXRobSI6IlNIQTI1NiJ9LHsibmFtZSI6ImJ1bmRsZS9odHRwL2V4YW1wbGUvYXV0aHovYXV0aHoucmVnbyIsImhhc2giOiI2MDJiZTcwMWIyYmE4ZTc3YTljNTNmOWIzM2QwZTkwM2MzNGMwMGMzMDkzM2Y2NDZiYmU3NGI3YzE2NGY2OGM2IiwiYWxnb3JpdGhtIjoiU0hBMjU2In0seyJuYW1lIjoiYnVuZGxlL3JvbGVzL2JpbmRpbmcvZGF0YS5qc29uIiwiaGFzaCI6ImIxODg1NTViZjczMGVlNDdkZjBiY2Y4MzVlYTNmNTQ1MjlmMzc4N2Y0ODQxZjFhZGE2MDM5M2RhYWZhZmJkYzciLCJhbGdvcml0aG0iOiJTSEEyNTYifV0sImtleWlkIjoiZm9vIiwic2NvcGUiOiJyZWFkIn0.YojuPnGWutdlDL7lwFGBXqPfDtxOG2BuZmShN5zm-G9zfMprI1AMqKDoPoNv4tuCGIBNXwoNsYHYiK538CHfJEfY1v4iDX3JFEWQlwx_CfJWDonwqT9SY9tHUW7PUUrI_WgJXZ5zei8RAMYMymKSb9hpSAtfGg_PU0kZr52WzjbPUj4SRiB19Swi61r0CFXYjbfx3GDJdjrGTNBSWrUCMrdhHYLEWqJPfSQ-fYfRrgQVhq3BJLwJJe66dgBEGnHEgA7XMuxkNIOv7mj3Y_EChbv2tjrD9NJPekDcYH1zCEc4BycHjNCcsGiQXDE6sFtoNZiCXLB2D0sLqUnBx4TCw27wTPfcOuL2KauLPahZitnH5mYvQD8NI76Pm4NSyJfevwdWjSsrT7vf0DCLS-dU6r9dJ79xM_hJU7136CT8ARcmSrk-EvCqfkrH2c4WwZyAzdyyyFumMZh4CYc2vcC7ap0NANHJT193fTud1i23mx1PBslwXdsIqXvBGlTbR7nb9o661m-B_mxbHMkG4nIeoGpZoaBJw8RVaA6-4D55gtk8aaMyLJIlIIlV2_AKOLk3nPG3ACHiLSndasLDOIRIYkCluIEaM2FLEEPEtJfKNR6e1K-EK2TvNKMDAEUtJW71ggOuGQ3b5otYOoVVENJLwm-PsO7qb2Tq6PyAquI3ExU`
	expected := make(map[string]interface{})
	expected["signatures"] = []string{token}

	files := map[string]string{}

	test.WithTempFS(files, func(rootDir string) {
		err := writeTokenToFile(token, rootDir)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		bs, err := os.ReadFile(filepath.Join(rootDir, ".signatures.json"))
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		expectedBytes, err := json.MarshalIndent(expected, "", " ")
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(expectedBytes, bs) {
			t.Fatal("Unexpected content in \".signatures.json\" file")
		}
	})
}

func TestDoSign(t *testing.T) {
	files := map[string]string{
		"foo/bar/data.json":     `{"y": 2}`,
		"/example/example.rego": `package example`,
		"/.signatures.json":     `{"signatures": []}`,
	}
	test.WithTempFS(files, func(rootDir string) {
		params := signCmdParams{
			algorithm:      "HS256",
			key:            "mysecret",
			outputFilePath: rootDir,
			bundleMode:     true,
		}

		err := doSign([]string{rootDir}, params)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}
	})
}

func TestBundleSignVerification(t *testing.T) {

	// files to be included in the bundle
	files := map[string]string{
		"/.manifest":            `{"revision": "quickbrownfaux"}`,
		"/a/b/c/data.json":      "[1,2,3]",
		"/a/b/d/data.json":      "true",
		"/a/b/y/data.yaml":      `foo: 1`,
		"/example/example.rego": `package example`,
		"/policy.wasm":          `modules-compiled-as-wasm-binary`,
		"/data.json":            `{"x": {"y": true}, "a": {"b": {"z": true}}}`,
	}

	test.WithTempFS(files, func(rootDir string) {
		params := signCmdParams{
			algorithm:      "HS256",
			key:            "mysecret",
			outputFilePath: rootDir,
			bundleMode:     true,
		}

		err := doSign([]string{rootDir}, params)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		// create gzipped tarball
		var filesInBundle [][2]string
		err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				bs, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				filesInBundle = append(filesInBundle, [2]string{path, string(bs)})
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		buf := archive.MustWriteTarGz(filesInBundle)

		// bundle verification config
		kc := keys.Config{
			Key:       "mysecret",
			Algorithm: "HS256",
		}

		bvc := bundle.NewVerificationConfig(map[string]*keys.Config{"foo": &kc}, "foo", "", nil)
		reader := bundle.NewReader(buf).WithBundleVerificationConfig(bvc).WithBaseDir(rootDir)

		_, err = reader.Read()
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}
	})
}

func TestValidateSignParams(t *testing.T) {

	tests := map[string]struct {
		args    []string
		params  signCmdParams
		wantErr bool
		err     error
	}{
		"no_args": {
			[]string{},
			newSignCmdParams(),
			true, fmt.Errorf("specify atleast one path containing policy and/or data files"),
		},
		"no_signing_key": {
			[]string{"foo"},
			newSignCmdParams(),
			true, fmt.Errorf("specify the secret (HMAC) or path of the PEM file containing the private key (RSA and ECDSA)"),
		},
		"empty_signing_key": {
			[]string{"foo"},
			signCmdParams{key: "", bundleMode: true},
			true, fmt.Errorf("specify the secret (HMAC) or path of the PEM file containing the private key (RSA and ECDSA)"),
		},
		"non_bundle_mode": {
			[]string{"foo"},
			signCmdParams{key: "foo"},
			true, fmt.Errorf("enable bundle mode (ie. --bundle) to sign bundle files or directories"),
		},
		"no_error": {
			[]string{"foo"},
			signCmdParams{key: "foo", bundleMode: true},
			false, nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			err := validateSignParams(tc.args, tc.params)

			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
	}
}
