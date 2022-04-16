// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
	"github.com/spf13/cobra"
)

type capabilitiesParams struct {
	capabilitiesFlag *capabilitiesFlag
	showVersions     bool
	showCurrent      bool
}

func newCapabilitiesParams() capabilitiesParams {
	return capabilitiesParams{
		capabilitiesFlag: newcapabilitiesFlag(),
	}
}

func init() {

	capabilitiesParams := newCapabilitiesParams()

	var capabilitiesCommand = &cobra.Command{
		Use:   "capabilities",
		Short: "Print the capabilities of OPA",
		Long: `Show capabilities for OPA.

The 'capabilities' command prints the OPA capabilities for a specific version.

Print a list of all existing capabilities versions

    $ opa capabilities --versions
    v0.17.0
    v0.17.1
    ...
    v0.37.1
    v0.37.2
    v0.38.0
    ...

Print the capabilities of a the current version in json

    $ opa capabilities --current
    {
        "builtins": [...],
        "future_keywords": [...],
        "wasm_abi_versions": [...]
    }

Print the capabilities of a specific version in json

    $ opa capabilities --capabilities v0.32.1
    {
        "builtins": [...],
        "future_keywords": null,
        "wasm_abi_versions": [...]
    }

`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, err := doCapabilities(capabilitiesParams)
			if err != nil {
				return err
			}
			fmt.Println(cs)
			return nil
		},
	}
	capabilitiesCommand.Flags().BoolVarP(&capabilitiesParams.showVersions, "versions", "", false, "list capabilities versions")
	capabilitiesCommand.Flags().BoolVarP(&capabilitiesParams.showCurrent, "current", "", false, "print current capabilities in json")

	addCapabilitiesFlag(capabilitiesCommand.Flags(), capabilitiesParams.capabilitiesFlag)

	// The version command can also be used to check for the latest released OPA version.
	// Some tools could use this for feature flagging purposes and hence this option is OFF by-default.
	// capabilitiesCommand.Flags().BoolVarP(&check, "check", "c", false, "check for latest OPA release")
	RootCommand.AddCommand(capabilitiesCommand)
}

func doCapabilities(params capabilitiesParams) (string, error) {
	if params.showVersions {
		cvs, err := ast.LoadCapabilitiesVersions()
		if err != nil {
			return "", err
		}

		var t string
		for _, cv := range cvs {
			t = t + cv + "\n"
		}
		return t, nil
	}

	var c *ast.Capabilities
	if params.capabilitiesFlag.C != nil {
		c = params.capabilitiesFlag.C
	} else if params.showCurrent {
		c = ast.CapabilitiesForThisVersion()
	}
	if c != nil {
		bs, err := util.MarshalJSON(c)
		if err != nil {
			return "", err
		}

		return string(bs), nil
	}
	return "", fmt.Errorf("please use a flag")
}
