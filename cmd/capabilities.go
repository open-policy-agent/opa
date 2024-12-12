// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/cmd/internal/env"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/spf13/cobra"
)

type capabilitiesParams struct {
	showCurrent  bool
	version      string
	file         string
	v0Compatible bool
}

func (p *capabilitiesParams) regoVersion() ast.RegoVersion {
	if p.v0Compatible {
		return ast.RegoV0
	}
	return ast.DefaultRegoVersion
}

func init() {

	capabilitiesParams := capabilitiesParams{}

	var capabilitiesCommand = &cobra.Command{
		Use:   "capabilities",
		Short: "Print the capabilities of OPA",
		Long: `Show capabilities for OPA.

The 'capabilities' command prints the OPA capabilities, prior to and including the version of OPA used.

Print a list of all existing capabilities version names

    $ opa capabilities
    v0.17.0
    v0.17.1
    ...
    v0.37.1
    v0.37.2
    v0.38.0
    ...

Print the capabilities of the current version

    $ opa capabilities --current
    {
        "builtins": [...],
        "future_keywords": [...],
        "wasm_abi_versions": [...]
    }

Print the capabilities of a specific version

    $ opa capabilities --version v0.32.1
    {
        "builtins": [...],
        "future_keywords": null,
        "wasm_abi_versions": [...]
    }

Print the capabilities of a capabilities file

    $ opa capabilities --file ./capabilities/v0.32.1.json
    {
        "builtins": [...],
        "future_keywords": null,
        "wasm_abi_versions": [...]
    }

`,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return env.CmdFlags.CheckEnvironmentVariables(cmd)
		},
		RunE: func(*cobra.Command, []string) error {
			cs, err := doCapabilities(capabilitiesParams)
			if err != nil {
				return err
			}
			fmt.Println(cs)
			return nil
		},
	}
	capabilitiesCommand.Flags().BoolVar(&capabilitiesParams.showCurrent, "current", false, "print current capabilities")
	capabilitiesCommand.Flags().StringVar(&capabilitiesParams.version, "version", "", "print capabilities of a specific version")
	capabilitiesCommand.Flags().StringVar(&capabilitiesParams.file, "file", "", "print capabilities defined by a file")
	addV0CompatibleFlag(capabilitiesCommand.Flags(), &capabilitiesParams.v0Compatible, false)

	RootCommand.AddCommand(capabilitiesCommand)
}

func doCapabilities(params capabilitiesParams) (string, error) {
	var (
		c   *ast.Capabilities
		err error
	)

	if len(params.version) > 0 {
		c, err = ast.LoadCapabilitiesVersion(params.version)
	} else if len(params.file) > 0 {
		c, err = ast.LoadCapabilitiesFile(params.file)
	} else if params.showCurrent {
		c = ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(params.regoVersion()))
	} else {
		return showVersions()
	}

	if err != nil {
		return "", err
	}

	bs, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bs), nil

}

func showVersions() (string, error) {
	cvs, err := ast.LoadCapabilitiesVersions()
	if err != nil {
		return "", err
	}

	t := strings.Join(cvs, "\n")
	return t, nil
}
