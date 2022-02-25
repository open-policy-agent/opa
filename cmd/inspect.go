// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	ib "github.com/open-policy-agent/opa/internal/bundle/inspect"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	iStrs "github.com/open-policy-agent/opa/internal/strings"
	"github.com/open-policy-agent/opa/util"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const maxTableFieldLen = 50

type inspectCommandParams struct {
	outputFormat      *util.EnumFlag
	listAnnotations   bool
	annotationsFilter []string
}

func newInspectCommandParams() inspectCommandParams {
	return inspectCommandParams{
		outputFormat: util.NewEnumFlag(evalPrettyOutput, []string{
			evalJSONOutput,
			evalPrettyOutput,
		}),
		listAnnotations:   false,
		annotationsFilter: []string{},
	}
}

func init() {

	params := newInspectCommandParams()

	var inspectCommand = &cobra.Command{
		Use:   "inspect <path> [<path> [...]]",
		Short: "Inspect OPA bundle(s)",
		Long: `Inspect OPA bundle(s).

The 'inspect' command provides a summary of the contents in OPA bundle(s). Bundles are
gzipped tarballs containing policies and data. The 'inspect' command reads bundle(s) and lists
the following:

* packages that are contributed by .rego files
* data locations defined by the data.json and data.yaml files
* manifest data
* signature data
* information about the Wasm module files
* package- and rule annotations

Example:

  $ ls
  bundle.tar.gz

  $ opa inspect bundle.tar.gz

You can provide exactly one OPA bundle or path to the 'inspect' command on the command-line. If you provide a path
referring to a directory, the 'inspect' command will load that path as a bundle and summarize its structure and contents.
`,
		PreRunE: func(_ *cobra.Command, args []string) error {
			return validateInspectParams(&params, args)
		},
		Run: func(_ *cobra.Command, args []string) {
			if err := doInspect(params, args[0], os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
		},
	}

	addOutputFormat(inspectCommand.Flags(), params.outputFormat)
	addListAnnotations(inspectCommand.Flags(), &params.listAnnotations)
	addAnnotationsFilter(inspectCommand.Flags(), &params.annotationsFilter)
	RootCommand.AddCommand(inspectCommand)
}

func doInspect(params inspectCommandParams, path string, out io.Writer) error {
	info, err := ib.File(path, params.listAnnotations, params.annotationsFilter)
	if err != nil {
		return err
	}

	switch params.outputFormat.String() {
	case evalJSONOutput:
		return pr.JSON(out, info)

	default:
		if info.Manifest.Revision != "" || len(*info.Manifest.Roots) != 0 || len(info.Manifest.Metadata) != 0 {
			if err := populateManifest(out, info.Manifest); err != nil {
				return err
			}
		}

		if len(info.Namespaces) != 0 {
			if err := populateNamespaces(out, info.Namespaces); err != nil {
				return err
			}
		}

		if params.listAnnotations && len(info.Annotations) != 0 {
			if err := populateAnnotations(out, info.Annotations); err != nil {
				return err
			}
		}

		return nil
	}
}

func validateInspectParams(p *inspectCommandParams, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("specify exactly one OPA bundle or path")
	}

	of := p.outputFormat.String()
	if of == evalJSONOutput || of == evalPrettyOutput {
		return nil
	}
	return fmt.Errorf("invalid output format for inspect command")
}

func populateManifest(out io.Writer, m bundle.Manifest) error {
	t := generateTableWithKeys(out, "field", "value")
	var lines [][]string

	if m.Revision != "" {
		lines = append(lines, []string{"Revision", truncateStr(m.Revision)})
	}

	if len(*m.Roots) != 0 {
		roots := *m.Roots
		if len(roots) == 1 {
			if roots[0] != "" {
				lines = append(lines, []string{"Roots", truncateFileName(roots[0])})
			}
		} else {
			sort.Strings(roots)
			for _, root := range roots {
				lines = append(lines, []string{"Roots", truncateFileName(root)})
			}
		}
	}

	if len(m.Metadata) != 0 {
		metadata, err := json.Marshal(m.Metadata)
		if err != nil {
			return err
		}
		lines = append(lines, []string{"Metadata", truncateStr(string(metadata))})
	}

	t.AppendBulk(lines)
	if t.NumLines() > 0 {
		fmt.Fprintln(out, "MANIFEST:")
		t.Render()
	}

	return nil
}

func populateNamespaces(out io.Writer, n map[string][]string) error {
	t := generateTableWithKeys(out, "namespace", "file")
	var lines [][]string

	var keys []string
	for k := range n {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		for _, file := range n[k] {
			lines = append(lines, []string{k, truncateFileName(file)})
		}
	}

	t.AppendBulk(lines)
	if t.NumLines() > 0 {
		fmt.Fprintln(out, "NAMESPACES:")
		t.Render()
	}

	return nil
}

func populateAnnotations(out io.Writer, as []*ast.AnnotationsRef) error {
	if len(as) > 0 {
		fmt.Fprintln(out, "ANNOTATIONS:")
		for _, a := range as {
			fmt.Fprintln(out, "Path:", a.Path.String())
			fmt.Fprintln(out, "Location:", a.Location.String())
			if len(a.Annotations.Title) > 0 {
				fmt.Fprintln(out, "Title:", a.Annotations.Title)
			}
			if len(a.Annotations.Description) > 0 {
				fmt.Fprintln(out, "Description:", a.Annotations.Description)
			}
			if len(a.Annotations.Organizations) > 0 {
				fmt.Fprintln(out, "Organizations:")
				for i, o := range a.Annotations.Organizations {
					fmt.Fprintf(out, " %d. %s\n", i+1, o)
				}
			}
			if len(a.Annotations.Authors) > 0 {
				fmt.Fprintln(out, "Authors:")
				for i, a := range a.Annotations.Authors {
					fmt.Fprintf(out, " %d. %s\n", i+1, a.CompactString())
				}
			}
			if len(a.Annotations.Schemas) > 0 {
				// NOTE(johanfylling): The Type Checker will MERGE all applicable schema annotations for a rule
				// into one list. Here, child nodes OVERRIDE parent nodes' schema annotations instead (default annot. behavior).
				// Should the former behavior be replicated here?
				fmt.Fprintln(out, "Schemas:")
				for i, s := range a.Annotations.Schemas {
					fmt.Fprintf(out, " %d. %s: ", i+1, s.Path.String())
					if len(s.Schema) > 0 {
						fmt.Fprintf(out, "%s\n", s.Schema.String())
					} else if s.Definition != nil {
						pr.JSON(out, s.Definition)
					}
				}
			}
			if len(a.Annotations.Custom) > 0 {
				fmt.Fprint(out, "Custom: ")
				if len(a.Annotations.Custom) > 0 {
					pr.JSON(out, a.Annotations.Custom)
				}
			}
			fmt.Fprintln(out, "----------")
		}
	}

	return nil
}

func generateTableWithKeys(writer io.Writer, keys ...string) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	aligns := []int{}
	var hdrs []string
	for _, k := range keys {
		hdrs = append(hdrs, strings.Title((k)))
		aligns = append(aligns, tablewriter.ALIGN_LEFT)
	}
	table.SetHeader(hdrs)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetColumnAlignment(aligns)
	table.SetAutoMergeCells(true)
	table.SetRowLine(false)
	table.SetAutoWrapText(false)
	return table
}

func truncateStr(s string) string {
	if len(s) < maxTableFieldLen {
		return s
	}
	return fmt.Sprintf("%v...", s[:maxTableFieldLen])
}

func truncateFileName(s string) string {
	if len(s) < maxTableFieldLen {
		return s
	}

	res, _ := iStrs.TruncateFilePaths(maxTableFieldLen, len(s), s)
	return res[s]
}
