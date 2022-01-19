package cmd

import (
    "github.com/withfig/autocomplete-tools/packages/cobra"
)

func init() {
    RootCommand.AddCommand(genFigSpec.NewCmdGenFigSpec())
}