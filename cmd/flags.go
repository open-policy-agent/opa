// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

func addConfigFileFlag(fs *pflag.FlagSet, file *string) {
	fs.StringVarP(file, "config-file", "c", "", "set path of configuration file")
}

func addConfigOverrides(fs *pflag.FlagSet, overrides *[]string) {
	fs.StringArrayVar(overrides, "set", []string{}, "override config values on the command line (use commas to specify multiple values)")
}

func addConfigOverrideFiles(fs *pflag.FlagSet, overrides *[]string) {
	fs.StringArrayVar(overrides, "set-file", []string{}, "override config values with files on the command line (use commas to specify multiple values)")
}

func addFailFlag(fs *pflag.FlagSet, fail *bool, value bool) {
	fs.BoolVarP(fail, "fail", "", value, "exits with non-zero exit code on undefined/empty result and errors")
}

func addDataFlag(fs *pflag.FlagSet, paths *repeatedStringFlag) {
	fs.VarP(paths, "data", "d", "set policy or data file(s). This flag can be repeated.")
}

func addBundleFlag(fs *pflag.FlagSet, paths *repeatedStringFlag) {
	fs.VarP(paths, "bundle", "b", "set bundle file(s) or directory path(s). This flag can be repeated.")
}

func addBundleModeFlag(fs *pflag.FlagSet, bundle *bool, value bool) {
	fs.BoolVarP(bundle, "bundle", "b", value, "load paths as bundle files or root directories")
}

func addInputFlag(fs *pflag.FlagSet, inputPath *string) {
	fs.StringVarP(inputPath, "input", "i", "", "set input file path")
}

func addImportFlag(fs *pflag.FlagSet, imports *repeatedStringFlag) {
	fs.VarP(imports, "import", "", "set query import(s). This flag can be repeated.")
}

func addPackageFlag(fs *pflag.FlagSet, pkg *string) {
	fs.StringVarP(pkg, "package", "", "", "set query package")
}

func addQueryStdinFlag(fs *pflag.FlagSet, stdin *bool) {
	fs.BoolVarP(stdin, "stdin", "", false, "read query from stdin")
}

func addInputStdinFlag(fs *pflag.FlagSet, stdinInput *bool) {
	fs.BoolVarP(stdinInput, "stdin-input", "I", false, "read input document from stdin")
}

func addMetricsFlag(fs *pflag.FlagSet, metrics *bool, value bool) {
	fs.BoolVarP(metrics, "metrics", "", value, "report query performance metrics")
}

func addOutputFormat(fs *pflag.FlagSet, outputFormat *util.EnumFlag) {
	fs.VarP(outputFormat, "format", "f", "set output format")
}

func addBenchmemFlag(fs *pflag.FlagSet, benchMem *bool, value bool) {
	fs.BoolVar(benchMem, "benchmem", value, "report memory allocations with benchmark results")
}

func addCountFlag(fs *pflag.FlagSet, count *int, cmdType string) {
	fs.IntVar(count, "count", 1, fmt.Sprintf("number of times to repeat each %s", cmdType))
}

func addMaxErrorsFlag(fs *pflag.FlagSet, errLimit *int) {
	fs.IntVarP(errLimit, "max-errors", "m", ast.CompileErrorLimitDefault, "set the number of errors to allow before compilation fails early")
}

func addIgnoreFlag(fs *pflag.FlagSet, ignoreNames *[]string) {
	fs.StringSliceVarP(ignoreNames, "ignore", "", []string{}, "set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)")
}

func addSigningAlgFlag(fs *pflag.FlagSet, alg *string, value string) {
	fs.StringVarP(alg, "signing-alg", "", value, "name of the signing algorithm")
}

func addClaimsFileFlag(fs *pflag.FlagSet, file *string) {
	fs.StringVarP(file, "claims-file", "", "", "set path of JSON file containing optional claims (see: https://openpolicyagent.org/docs/latest/management/#signature-format)")
}

func addSigningKeyFlag(fs *pflag.FlagSet, key *string) {
	fs.StringVarP(key, "signing-key", "", "", "set the secret (HMAC) or path of the PEM file containing the private key (RSA and ECDSA)")
}

func addSigningPluginFlag(fs *pflag.FlagSet, plugin *string) {
	fs.StringVarP(plugin, "signing-plugin", "", "", "name of the plugin to use for signing/verification (see https://openpolicyagent.org/docs/latest/management/#signature-plugin")
}

func addVerificationKeyFlag(fs *pflag.FlagSet, key *string) {
	fs.StringVarP(key, "verification-key", "", "", "set the secret (HMAC) or path of the PEM file containing the public key (RSA and ECDSA)")
}

func addVerificationKeyIDFlag(fs *pflag.FlagSet, keyID *string, value string) {
	fs.StringVarP(keyID, "verification-key-id", "", value, "name assigned to the verification key used for bundle verification")
}

func addBundleVerificationScopeFlag(fs *pflag.FlagSet, scope *string) {
	fs.StringVarP(scope, "scope", "", "", "scope to use for bundle signature verification")
}

func addBundleVerificationSkipFlag(fs *pflag.FlagSet, skip *bool, value bool) {
	fs.BoolVarP(skip, "skip-verify", "", value, "disables bundle signature verification")
}

func addBundleVerificationExcludeFilesFlag(fs *pflag.FlagSet, excludeNames *[]string) {
	fs.StringSliceVarP(excludeNames, "exclude-files-verify", "", []string{}, "set file names to exclude during bundle verification")
}

func addCapabilitiesFlag(fs *pflag.FlagSet, f *capabilitiesFlag) {
	fs.VarP(f, "capabilities", "", "set capabilities.json file path")
}

func addPartialFlag(fs *pflag.FlagSet, partial *bool, value bool) {
	fs.BoolVarP(partial, "partial", "p", value, "perform partial evaluation")
}

func addUnknownsFlag(fs *pflag.FlagSet, unknowns *[]string, value []string) {
	fs.StringArrayVarP(unknowns, "unknowns", "u", value, "set paths to treat as unknown during partial evaluation")
}

func addSchemaFlag(fs *pflag.FlagSet, schemaPath *string) {
	fs.StringVarP(schemaPath, "schema", "s", "", "set schema file path or directory path")
}

func addTargetFlag(fs *pflag.FlagSet, target *util.EnumFlag) {
	fs.VarP(target, "target", "t", "set the runtime to exercise")
}

const (
	explainModeOff   = "off"
	explainModeFull  = "full"
	explainModeNotes = "notes"
	explainModeFails = "fails"
)

func newExplainFlag(modes []string) *util.EnumFlag {
	return util.NewEnumFlag(modes[0], modes)
}

func setExplainFlag(fs *pflag.FlagSet, explain *util.EnumFlag) {
	fs.VarP(explain, "explain", "", "enable query explanations")
}

type capabilitiesFlag struct {
	C    *ast.Capabilities
	path string
}

func newcapabilitiesFlag() *capabilitiesFlag {
	return &capabilitiesFlag{
		// cannot call ast.CapabilitiesForThisVersion here because
		// custom builtins cannot be registered by this point in execution
		C: nil,
	}
}

func (f *capabilitiesFlag) Type() string {
	return "string"
}

func (f *capabilitiesFlag) String() string {
	return f.path
}

func (f *capabilitiesFlag) Set(s string) error {
	f.path = s
	fd, err := os.Open(s)
	if err != nil {
		return err
	}
	defer fd.Close()
	f.C, err = ast.LoadCapabilitiesJSON(fd)
	return err
}
