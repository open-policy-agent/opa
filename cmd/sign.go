// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/bundle"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/util"
)

type signCmdParams struct {
	algorithm      string
	key            string
	claimsFile     string
	outputFilePath string
	bundleMode     bool
	plugin         string
}

const (
	defaultTokenSigningAlg = "RS256"
	defaultHashingAlg      = "SHA-256"
	signaturesFile         = ".signatures.json"
)

func newSignCmdParams() signCmdParams {
	return signCmdParams{}
}

func init() {
	cmdParams := newSignCmdParams()

	var signCommand = &cobra.Command{
		Use:   "sign <path> [<path> [...]]",
		Short: "Generate an OPA bundle signature",
		Long: `Generate an OPA bundle signature.

The 'sign' command generates a digital signature for policy bundles. It generates a
".signatures.json" file that dictates which files should be included in the bundle,
what their SHA hashes are, and is cryptographically secure.

The signatures file is a JSON file with an array containing a single JSON Web Token (JWT)
that encapsulates the signature for the bundle.

The --signing-alg flag can be used to specify the algorithm to sign the token. The 'sign'
command uses RS256 (by default) as the signing algorithm.
See https://www.openpolicyagent.org/docs/latest/configuration/#keys
for a list of supported signing algorithms.

The key to be used for signing the JWT MUST be provided using the --signing-key flag.
For example, for RSA family of algorithms, the command expects a PEM file containing
the private key.
For HMAC family of algorithms (eg. HS256), the secret can be provided using
the --signing-key flag.

OPA 'sign' can ONLY be used with the --bundle flag to load paths that refer to
existing bundle files or directories following the bundle structure.

	$ opa sign --signing-key /path/to/private_key.pem --bundle foo

Where foo has the following structure:

	foo/
	  |
	  +-- bar/
	  |     |
	  |     +-- data.json
	  |
	  +-- policy.rego
	  |
	  +-- .manifest

This will create a ".signatures.json" file in the current directory.
The --output-file-path flag can be used to specify a different location for
the ".signatures.json" file.

The content of the ".signatures.json" file is shown below:

	{
	  "signatures": [
		"eyJhbGciOiJSUzI1NiJ9.eyJmaWxlcyI6W3sibmFtZSI6Ii5tYW5pZmVzdCIsImhhc2giOiIxODc0NWRlNzJjMDFlODBjZDlmNTIwZjQxOGMwMDlhYzRkMmMzZDAyYjE3YTUwZTJkMDQyMTU4YmMzNTJhMzJkIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6ImJhci9kYXRhLmpzb24iLCJoYXNoIjoiOTNhMjM5NzFhOTE0ZTVlYWNiZjBhOGQyNTE1NGNkYTMwOWMzYzFjNzJmYmI5OTE0ZDQ3YzYwZjNjYjY4MTU4OCIsImFsZ29yaXRobSI6IlNIQS0yNTYifSx7Im5hbWUiOiJwb2xpY3kucmVnbyIsImhhc2giOiJkMGYyNDJhYWUzNGRiNTRlZjU2NmJlYTRkNDVmY2YxOTcwMGM1ZDhmODdhOWRiOTMyZGZhZDZkMWYwZjI5MWFjIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9XX0.lNsmRqrmT1JI4Z_zpY6IzHRZQAU306PyOjZ6osquixPuTtdSBxgbsdKDcp7Civw3B77BgygVsvx4k3fYr8XCDKChm0uYKScrpFr9_yS6g5mVTQws3KZncZXCQHdupRFoqMS8vXAVgJr52C83AinYWABwH2RYq_B0ZPf_GDzaMgzpep9RlDNecGs57_4zlyxmP2ESU8kjfX8jAA6rYFKeGXJHMD-j4SassoYIzYRv9YkHx8F8Y2ae5Kd5M24Ql0kkvqc_4eO_T9s4nbQ4q5qGHGE-91ND1KVn2avcUyVVPc0-XCR7EH8HnHgCl0v1c7gX1RL7ET7NJbPzfmzQAzk0ZW0dEHI4KZnXSpqy8m-3zAc8kIARm2QwoNEWpy3MWiooPeZVSa9d5iw1aLrbyumfjBP0vCQEPes-Aa6PrARwd5jR9SacO5By0-4emzskvJYRZqbfJ9tXSXDMcAFOAm6kqRPJaj8AO4CyajTC_Lt32_0OLeXqYgNpt3HDqLqGjrb-8fVeQc-hKh0aES8XehQqXj4jMwfsTyj5alsXZm08LwzcFlfQZ7s1kUtmr0_BBNJYcdZUdlu6Qio3LFSRYXNuu6edAO1VH5GKqZISvE1uvDZb2E0Z-rtH-oPp1iSpfvsX47jKJ42LVpI6OahEBri44dzHOIwwm3CIuV8gFzOwR0k"
	  ]
	}

And the decoded JWT payload has the following form:

	{
	  "files": [
		{
		  "name": ".manifest",
		  "hash": "18745de72c01e80cd9f520f418c009ac4d2c3d02b17a50e2d042158bc352a32d",
		  "algorithm": "SHA-256"
		},
		{
		  "name": "policy.rego",
		  "hash": "d0f242aae34db54ef566bea4d45fcf19700c5d8f87a9db932dfad6d1f0f291ac",
		  "algorithm": "SHA-256"
		},
		{
		  "name": "bar/data.json",
		  "hash": "93a23971a914e5eacbf0a8d25154cda309c3c1c72fbb9914d47c60f3cb681588",
		  "algorithm": "SHA-256"
		}
	  ]
	}

The "files" field is generated from the files under the directory path(s)
provided to the 'sign' command. During bundle signature verification, OPA will check
each file name (ex. "foo/bar/data.json") in the "files" field
exists in the actual bundle. The file content is hashed using SHA256.

To include additional claims in the payload use the --claims-file flag to provide
a JSON file containing optional claims.

For more information on the format of the ".signatures.json" file see
https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-format.
`,
		PreRunE: func(Cmd *cobra.Command, args []string) error {
			return validateSignParams(args, cmdParams)
		},

		Run: func(cmd *cobra.Command, args []string) {
			if err := doSign(args, cmdParams); err != nil {
				fmt.Println("error:", err)
				os.Exit(1)
			}
		},
	}

	addBundleModeFlag(signCommand.Flags(), &cmdParams.bundleMode, false)

	// bundle signing config
	addSigningKeyFlag(signCommand.Flags(), &cmdParams.key)
	addClaimsFileFlag(signCommand.Flags(), &cmdParams.claimsFile)
	addSigningAlgFlag(signCommand.Flags(), &cmdParams.algorithm, defaultTokenSigningAlg)
	addSigningPluginFlag(signCommand.Flags(), &cmdParams.plugin)

	signCommand.Flags().StringVarP(&cmdParams.outputFilePath, "output-file-path", "o", ".", "set the location for the .signatures.json file")

	RootCommand.AddCommand(signCommand)
}

func doSign(args []string, params signCmdParams) error {
	load, err := initload.WalkPaths(args, nil, params.bundleMode)
	if err != nil {
		return err
	}

	hash, err := bundle.NewSignatureHasher(bundle.HashingAlgorithm(defaultHashingAlg))
	if err != nil {
		return err
	}

	files, err := readBundleFiles(load.BundlesLoader, hash)
	if err != nil {
		return err
	}

	signingConfig := buildSigningConfig(params.key, params.algorithm, params.claimsFile, params.plugin)

	token, err := bundle.GenerateSignedToken(files, signingConfig, "")
	if err != nil {
		return err
	}

	return writeTokenToFile(token, params.outputFilePath)
}

func readBundleFiles(loaders []initload.BundleLoader, h bundle.SignatureHasher) ([]bundle.FileInfo, error) {
	files := []bundle.FileInfo{}

	for _, bl := range loaders {
		for {
			f, err := bl.DirectoryLoader.NextFile()
			if err == io.EOF {
				break
			}

			if err != nil {
				return files, fmt.Errorf("bundle read failed: %w", err)
			}

			// skip existing signatures file
			if strings.HasSuffix(f.Path(), bundle.SignaturesFile) {
				continue
			}

			var buf bytes.Buffer
			n, err := f.Read(&buf, bundle.DefaultSizeLimitBytes+1)
			f.Close()

			if err != nil && err != io.EOF {
				return files, err
			} else if err == nil && n >= bundle.DefaultSizeLimitBytes {
				return files, fmt.Errorf("bundle file exceeded max size (%v bytes)", bundle.DefaultSizeLimitBytes)
			}

			path := f.Path()
			if bl.IsDir {
				path = f.URL()
			}

			// hash the file content
			fi, err := hashFileContent(h, buf.Bytes(), path)
			if err != nil {
				return files, err
			}
			files = append(files, fi)
		}
	}
	return files, nil
}

func hashFileContent(h bundle.SignatureHasher, data []byte, path string) (bundle.FileInfo, error) {

	var fileInfo bundle.FileInfo
	var value interface{}

	if bundle.IsStructuredDoc(path) {
		err := util.Unmarshal(data, &value)
		if err != nil {
			return fileInfo, err
		}
	} else {
		value = data
	}

	bytes, err := h.HashFile(value)
	if err != nil {
		return fileInfo, err
	}

	return bundle.NewFile(strings.TrimPrefix(path, "/"), hex.EncodeToString(bytes), defaultHashingAlg), nil
}

func writeTokenToFile(token, fileLoc string) error {
	content := make(map[string]interface{})
	content["signatures"] = []string{token}

	bs, err := json.MarshalIndent(content, "", " ")
	if err != nil {
		return err
	}

	path := signaturesFile
	if fileLoc != "" {
		path = filepath.Join(fileLoc, path)
	}
	return ioutil.WriteFile(path, bs, 0644)
}

func validateSignParams(args []string, params signCmdParams) error {
	if len(args) == 0 {
		return fmt.Errorf("specify atleast one path containing policy and/or data files")
	}

	if params.key == "" {
		return fmt.Errorf("specify the secret (HMAC) or path of the PEM file containing the private key (RSA and ECDSA)")
	}

	if !params.bundleMode {
		return fmt.Errorf("enable bundle mode (ie. --bundle) to sign bundle files or directories")
	}
	return nil
}
