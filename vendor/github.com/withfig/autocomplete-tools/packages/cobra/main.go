/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package genFigSpec

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var includeHidden bool

var generateCommandArgs func(*cobra.Command) Args

type Opts struct {
	Use                 string
	Short               string
	Visible             bool
	Long                string
	commandArgGenerator func(*cobra.Command) Args
}

func NewCmdGenFigSpec(options ...Opts) *cobra.Command {
	Use := "generateFigSpec"
	Aliases := []string{"genFigSpec"}
	Short := "Generate a fig spec"
	Hidden := true
	Long := `
Fig is a tool for your command line that adds autocomplete.
This command generates a TypeScript file with the skeleton
Fig autocomplete spec for your Cobra CLI.
`
	if len(options) > 0 {
		if options[0].Use != "" {
			Use = options[0].Use
		}
		if options[0].Short != "" {
			Short = options[0].Short
		}
		if options[0].Long != "" {
			Long = options[0].Long
		}
		if options[0].Visible {
			Hidden = false
		}
		if options[0].commandArgGenerator != nil {
			generateCommandArgs = options[0].commandArgGenerator
		}
	}
	var cmd = &cobra.Command{
		Use:    Use,
		Aliases: Aliases,
		Short:  Short,
		Hidden: Hidden,
		Long:   Long,
		Run: func(cmd *cobra.Command, args []string) {
			root := cmd.Root()
			spec := MakeFigSpec(root)
			fmt.Println(spec.toTypescript())
		},
	}
	cmd.Flags().BoolVar(
		&includeHidden, "include-hidden", false,
		"Include hidden commands in generated Fig autocomplete spec")
	return cmd
}

func MakeFigSpec(root *cobra.Command) Spec {
	opts := append(options(root.InheritedFlags()), options(root.NonInheritedFlags())...)
	opts = append(opts, makeHelpOption(root.Name()))
	spec := Spec{
		Subcommand: &Subcommand{
			BaseSuggestion: &BaseSuggestion{
				description: root.Short,
			},
			options:     opts,
			subcommands: append(subcommands(root), makeHelpCommand(root)), // We assume CLI is using default help command
			args:        commandArguments(root),
		},
		name: root.Name(),
	}
	return spec
}

func subcommands(cmd *cobra.Command) Subcommands {
	return _subcommands(cmd, false, []Option{})
}

func _subcommands(cmd *cobra.Command, overrideOptions bool, overrides Options) Subcommands {
	var subs []Subcommand
	for _, sub := range cmd.Commands() {
		if sub.Name() == "help" || (!includeHidden && sub.Hidden) {
			continue
		}
		var opts Options
		if overrideOptions {
			opts = overrides
		} else {
			opts = append(options(sub.InheritedFlags()), options(sub.NonInheritedFlags())...)
		}
		opts = append(opts, makeHelpOption(sub.Name())) // We assume every command has access to the default --help flag
		subs = append(subs, Subcommand{
			BaseSuggestion: &BaseSuggestion{
				description: sub.Short,
			},
			name:        append(sub.Aliases, sub.Name()),
			options:     opts,
			subcommands: subcommands(sub),
			args:        commandArguments(sub),
		})
	}
	return subs
}

func options(flagSet *pflag.FlagSet) []Option {
	var opts []Option
	attachFlags := func(flag *pflag.Flag) {

		option := Option{
			BaseSuggestion: &BaseSuggestion{
				displayName: flag.Name,
				description: flag.Usage,
			},
			name:         []string{fmt.Sprintf("--%v", flag.Name)},
			isRepeatable: strings.Contains(strings.ToLower(flag.Value.Type()), "array"),
		}
		if flag.Shorthand != "" {
			option.name = append(option.name, fmt.Sprintf("-%v", flag.Shorthand))
		}
		requiredAnnotation, found := flag.Annotations[cobra.BashCompOneRequiredFlag]
		if found && requiredAnnotation[0] == "true" {
			option.isRequired = true
		}
		option.args = flagArguments(flag)
		opts = append(opts, option)
	}

	flagSet.VisitAll(attachFlags)
	return opts
}

/*
 * In Cobra, you only specify the number of arguments.
 * Not sure how we want to handle this (if at all)
 * https://github.com/spf13/cobra/blob/v1.2.1/user_guide.md#positional-and-custom-arguments
 */
func commandArguments(cmd *cobra.Command) []Arg {
	if generateCommandArgs != nil {
		return generateCommandArgs(cmd)
	}
	return []Arg{}
}

func flagArguments(flag *pflag.Flag) []Arg {
	var args []Arg
	defaultVal := flag.DefValue
	if defaultVal == "[]" {
		defaultVal = ""
	}
	if flag.Value.Type() != "bool" {
		arg := Arg{
			name:       flag.Name,
			defaultVal: defaultVal,
		}
		_, foundFilenameAnnotation := flag.Annotations[cobra.BashCompFilenameExt]
		if foundFilenameAnnotation {
			arg.template = append(arg.template, FILEPATHS)
		}
		_, foundDirectoryAnnotation := flag.Annotations[cobra.BashCompSubdirsInDir]
		if foundDirectoryAnnotation {
			arg.template = append(arg.template, FOLDERS)
		}
		args = append(args, arg)
	}
	return args
}

func makeHelpCommand(root *cobra.Command) Subcommand {
	return Subcommand{
		BaseSuggestion: &BaseSuggestion{
			description: "Help about any command",
		},
		name:        []string{"help"},
		options:     append(options(root.PersistentFlags()), makeHelpOption("help")),
		subcommands: _subcommands(root, true, options(root.PersistentFlags())),
	}
}

func makeHelpOption(commandName string) Option {
	return Option{
		BaseSuggestion: &BaseSuggestion{
			displayName: "help",
			description: fmt.Sprintf("help for %v", commandName),
		},
		name: []string{"--help", "-h"},
	}
}
