package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/open-policy-agent/opa/cmd"
)

func main() {
	command := cmd.Command(nil, "opa")
	command.Use = "opa [command]"
	command.DisableAutoGenTag = true

	cmdData := make([]map[string]any, 0)

	for _, c := range command.Commands() {
		if !showCommand(c) {
			continue
		}

		cmdData = append(cmdData, cmdToData(c))
	}

	err := json.NewEncoder(os.Stdout).Encode(cmdData)
	if err != nil {
		log.Fatal(err)
	}
}

func showCommand(c *cobra.Command) bool {
	if !c.IsAvailableCommand() ||
		c.IsAdditionalHelpTopicCommand() ||
		c.Hidden {
		return false
	}

	return true
}

func cmdToID(c *cobra.Command) string {
	parts := strings.Split(c.Use, " ")

	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

func extractFlags(flagSet *pflag.FlagSet) []map[string]any {
	var result []map[string]any

	flagSet.VisitAll(func(f *pflag.Flag) {
		flagInfo := map[string]any{
			"name":        "--" + f.Name,
			"shorthand":   "",
			"type":        f.Value.Type(),
			"default":     f.DefValue,
			"description": f.Usage,
		}

		if f.Shorthand != "" {
			flagInfo["shorthand"] = "-" + f.Shorthand
		}

		result = append(result, flagInfo)
	})

	return result
}

func cmdToData(c *cobra.Command) map[string]any {
	childData := make([]map[string]any, 0)
	for _, childCmd := range c.Commands() {
		if !showCommand(childCmd) {
			continue
		}
		childData = append(childData, cmdToData(childCmd))
	}

	return map[string]any{
		"id":           cmdToID(c),
		"use":          c.Use,
		"useline":      c.UseLine(),
		"short":        c.Short,
		"long":         c.Long,
		"example":      c.Example,
		"flags":        extractFlags(c.NonInheritedFlags()),
		"parent_flags": extractFlags(c.InheritedFlags()),
		"children":     childData,
	}
}
