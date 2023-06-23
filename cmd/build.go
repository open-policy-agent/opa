// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/keys"
	"github.com/open-policy-agent/opa/util"
)

const defaultPublicKeyID = "default"

type buildParams struct {
	capabilities       *capabilitiesFlag
	target             *util.EnumFlag
	bundleMode         bool
	pruneUnused        bool
	optimizationLevel  int
	entrypoints        repeatedStringFlag
	outputFile         string
	revision           stringptrFlag
	ignore             []string
	debug              bool
	algorithm          string
	key                string
	scope              string
	pubKey             string
	pubKeyID           string
	claimsFile         string
	excludeVerifyFiles []string
	plugin             string
	ns                 string
}

func newBuildParams() buildParams {
	return buildParams{
		capabilities: newcapabilitiesFlag(),
		target:       util.NewEnumFlag(compile.TargetRego, compile.Targets),
	}
}

func init() {

	buildParams := newBuildParams()

	var buildCommand = &cobra.Command{
		Use:   "build <path> [<path> [...]]",
		Short: "Build an OPA bundle",
		Long: `Build an OPA bundle.

The 'build' command packages OPA policy and data files into bundles. Bundles are
gzipped tarballs containing policies and data. Paths referring to directories are
loaded recursively.

    $ ls
    example.rego

    $ opa build -b .

You can load bundles into OPA on the command-line:

    $ ls
    bundle.tar.gz example.rego

    $ opa run bundle.tar.gz

You can also configure OPA to download bundles from remote HTTP endpoints:

    $ opa run --server \
        --set bundles.example.resource=bundle.tar.gz \
        --set services.example.url=http://localhost:8080

Inside another terminal in the same directory, serve the bundle via HTTP:

    $ python3 -m http.server --bind localhost 8080

For more information on bundles see https://www.openpolicyagent.org/docs/latest/management-bundles/.

Common Flags
------------

When -b is specified the 'build' command assumes paths refer to existing bundle files
or directories following the bundle structure. If multiple bundles are provided, their
contents are merged. If there are any merge conflicts (e.g., due to conflicting bundle
roots), the command fails. When loading an existing bundle file, the .manifest from
the input bundle will be included in the output bundle. Flags that set .manifest fields
(such as --revision) override input bundle .manifest fields.

The -O flag controls the optimization level. By default, optimization is disabled (-O=0).
When optimization is enabled the 'build' command generates a bundle that is semantically
equivalent to the input files however the structure of the files in the bundle may have
been changed by rewriting, inlining, pruning, etc. Higher optimization levels may result
in longer build times. The --partial-namespace flag can used in conjunction with the -O flag
to specify the namespace for the partially evaluated files in the optimized bundle.

The 'build' command supports targets (specified by -t):

    rego    The default target emits a bundle containing a set of policy and data files
            that are semantically equivalent to the input files. If optimizations are
            disabled the output may simply contain a copy of the input policy and data
            files. If optimization is enabled at least one entrypoint must be supplied,
            either via the -e option, or via entrypoint metadata annotations.

    wasm    The wasm target emits a bundle containing a WebAssembly module compiled from
            the input files for each specified entrypoint. The bundle may contain the
            original policy or data files.

    plan    The plan target emits a bundle containing a plan, i.e., an intermediate
            representation compiled from the input files for each specified entrypoint.
            This is for further processing, OPA cannot evaluate a "plan bundle" like it
            can evaluate a wasm or rego bundle.

The -e flag tells the 'build' command which documents (entrypoints) will be queried by 
the software asking for policy decisions, so that it can focus optimization efforts and 
ensure that document is not eliminated by the optimizer.
Note: Unless the --prune-unused flag is used, any rule transitively referring to a 
package or rule declared as an entrypoint will also be enumerated as an entrypoint.

Signing
-------

The 'build' command can be used to verify the signature of a signed bundle and
also to generate a signature for the output bundle the command creates.

If the directory path(s) provided to the 'build' command contain a ".signatures.json" file,
it will attempt to verify the signatures included in that file. The bundle files
or directory path(s) to verify must be specified using --bundle.

For more information on the bundle signing and verification, see
https://www.openpolicyagent.org/docs/latest/management-bundles/#signing.

Example:

    $ opa build --verification-key /path/to/public_key.pem --signing-key /path/to/private_key.pem --bundle foo

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
      |
      +-- .signatures.json


The 'build' command will verify the signatures using the public key provided by the --verification-key flag.
The default signing algorithm is RS256 and the --signing-alg flag can be used to specify
a different one. The --verification-key-id and --scope flags can be used to specify the name for the key
provided using the --verification-key flag and scope to use for bundle signature verification respectively.

If the verification succeeds, the 'build' command will write out an updated ".signatures.json" file
to the output bundle. It will use the key specified by the --signing-key flag to sign
the token in the ".signatures.json" file.

To include additional claims in the payload use the --claims-file flag to provide a JSON file
containing optional claims.

For more information on the format of the ".signatures.json" file
see https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-format.

Capabilities
------------

The 'build' command can validate policies against a configurable set of OPA capabilities.
The capabilities define the built-in functions and other language features that policies
may depend on. For example, the following capabilities file only permits the policy to
depend on the "plus" built-in function ('+'):

    {
        "builtins": [
            {
                "name": "plus",
                "infix": "+",
                "decl": {
                    "type": "function",
                    "args": [
                        {
                            "type": "number"
                        },
                        {
                            "type": "number"
                        }
                    ],
                    "result": {
                        "type": "number"
                    }
                }
            }
        ]
    }

Capabilities can be used to validate policies against a specific version of OPA.
The OPA repository contains a set of capabilities files for each OPA release. For example,
the following command builds a directory of policies ('./policies') and validates them
against OPA v0.22.0:

    opa build ./policies --capabilities v0.22.0
`,
		PreRunE: func(Cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("expected at least one path")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := dobuild(buildParams, args); err != nil {
				fmt.Println("error:", err)
				os.Exit(1)
			}
		},
	}

	buildCommand.Flags().VarP(buildParams.target, "target", "t", "set the output bundle target type")
	buildCommand.Flags().BoolVar(&buildParams.pruneUnused, "prune-unused", false, "exclude dependents of entrypoints")
	buildCommand.Flags().BoolVar(&buildParams.debug, "debug", false, "enable debug output")
	buildCommand.Flags().IntVarP(&buildParams.optimizationLevel, "optimize", "O", 0, "set optimization level")
	buildCommand.Flags().VarP(&buildParams.entrypoints, "entrypoint", "e", "set slash separated entrypoint path")
	buildCommand.Flags().VarP(&buildParams.revision, "revision", "r", "set output bundle revision")
	buildCommand.Flags().StringVarP(&buildParams.outputFile, "output", "o", "bundle.tar.gz", "set the output filename")
	buildCommand.Flags().StringVar(&buildParams.ns, "partial-namespace", "partial", "set the namespace to use for partially evaluated files in an optimized bundle")

	addBundleModeFlag(buildCommand.Flags(), &buildParams.bundleMode, false)
	addIgnoreFlag(buildCommand.Flags(), &buildParams.ignore)
	addCapabilitiesFlag(buildCommand.Flags(), buildParams.capabilities)

	// bundle verification config
	addVerificationKeyFlag(buildCommand.Flags(), &buildParams.pubKey)
	addVerificationKeyIDFlag(buildCommand.Flags(), &buildParams.pubKeyID, defaultPublicKeyID)
	addSigningAlgFlag(buildCommand.Flags(), &buildParams.algorithm, defaultTokenSigningAlg)
	addBundleVerificationScopeFlag(buildCommand.Flags(), &buildParams.scope)
	addBundleVerificationExcludeFilesFlag(buildCommand.Flags(), &buildParams.excludeVerifyFiles)

	// bundle signing config
	addSigningKeyFlag(buildCommand.Flags(), &buildParams.key)
	addSigningPluginFlag(buildCommand.Flags(), &buildParams.plugin)
	addClaimsFileFlag(buildCommand.Flags(), &buildParams.claimsFile)

	RootCommand.AddCommand(buildCommand)
}

func dobuild(params buildParams, args []string) error {

	buf := bytes.NewBuffer(nil)

	// generate the bundle verification and signing config
	bvc, err := buildVerificationConfig(params.pubKey, params.pubKeyID, params.algorithm, params.scope, params.excludeVerifyFiles)
	if err != nil {
		return err
	}

	bsc, err := buildSigningConfig(params.key, params.algorithm, params.claimsFile, params.plugin)
	if err != nil {
		return err
	}

	if (bvc != nil || bsc != nil) && !params.bundleMode {
		return fmt.Errorf("enable bundle mode (ie. --bundle) to verify or sign bundle files or directories")
	}

	var capabilities *ast.Capabilities
	// if capabilities are not provided as a cmd flag,
	// then ast.CapabilitiesForThisVersion must be called
	// within dobuild to ensure custom builtins are properly captured
	if params.capabilities.C != nil {
		capabilities = params.capabilities.C
	} else {
		capabilities = ast.CapabilitiesForThisVersion()
	}

	compiler := compile.New().
		WithCapabilities(capabilities).
		WithTarget(params.target.String()).
		WithAsBundle(params.bundleMode).
		WithPruneUnused(params.pruneUnused).
		WithOptimizationLevel(params.optimizationLevel).
		WithOutput(buf).
		WithEntrypoints(params.entrypoints.v...).
		WithRegoAnnotationEntrypoints(true).
		WithPaths(args...).
		WithFilter(buildCommandLoaderFilter(params.bundleMode, params.ignore)).
		WithBundleVerificationConfig(bvc).
		WithBundleSigningConfig(bsc).
		WithPartialNamespace(params.ns)

	if params.revision.isSet {
		compiler = compiler.WithRevision(*params.revision.v)
	}

	if params.debug {
		compiler = compiler.WithDebug(os.Stderr)
	}

	if params.claimsFile == "" {
		compiler = compiler.WithBundleVerificationKeyID(params.pubKeyID)
	}

	if params.target.String() == compile.TargetPlan {
		compiler = compiler.WithEnablePrintStatements(true)
	}

	err = compiler.Build(context.Background())
	if err != nil {
		return err
	}

	out, err := os.Create(params.outputFile)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, buf)
	if err != nil {
		return err
	}

	return out.Close()
}

func buildCommandLoaderFilter(bundleMode bool, ignore []string) func(string, os.FileInfo, int) bool {
	return func(abspath string, info os.FileInfo, depth int) bool {
		if !bundleMode {
			if !info.IsDir() && strings.HasSuffix(abspath, ".tar.gz") {
				return true
			}
		}
		return loaderFilter{Ignore: ignore}.Apply(abspath, info, depth)
	}
}

func buildVerificationConfig(pubKey, pubKeyID, alg, scope string, excludeFiles []string) (*bundle.VerificationConfig, error) {
	if pubKey == "" {
		return nil, nil
	}

	keyConfig, err := keys.NewKeyConfig(pubKey, alg, scope)
	if err != nil {
		return nil, err
	}
	return bundle.NewVerificationConfig(map[string]*keys.Config{pubKeyID: keyConfig}, pubKeyID, scope, excludeFiles), nil
}

func buildSigningConfig(key, alg, claimsFile, plugin string) (*bundle.SigningConfig, error) {
	if key == "" && (plugin != "" || claimsFile != "") {
		return nil, errSigningConfigIncomplete
	}
	if key == "" {
		return nil, nil
	}
	return bundle.NewSigningConfig(key, alg, claimsFile).WithPlugin(plugin), nil
}
