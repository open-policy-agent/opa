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
const pageWidth = 80

type inspectCommandParams struct {
	outputFormat    *util.EnumFlag
	listAnnotations bool
}

func newInspectCommandParams() inspectCommandParams {
	return inspectCommandParams{
		outputFormat: util.NewEnumFlag(evalPrettyOutput, []string{
			evalJSONOutput,
			evalPrettyOutput,
		}),
		listAnnotations: false,
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
	RootCommand.AddCommand(inspectCommand)
}

func doInspect(params inspectCommandParams, path string, out io.Writer) error {
	info, err := ib.File(path, params.listAnnotations)
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
		lines = append(lines, []string{"Revision", truncateTableStr(m.Revision)})
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
		lines = append(lines, []string{"Metadata", truncateTableStr(string(metadata))})
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
	// only auto-merge the namespace column
	t.SetAutoMergeCells(false)
	t.SetAutoMergeCellsByColumnIndex([]int{0})
	var lines [][]string

	keys := make([]string, 0, len(n))
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

func populateAnnotations(out io.Writer, refs []*ast.AnnotationsRef) error {
	if len(refs) > 0 {
		fmt.Fprintln(out, "ANNOTATIONS:")
		for _, ref := range refs {
			printTitle(out, ref)
			fmt.Fprintln(out)

			if a := ref.Annotations; a != nil && len(a.Description) > 0 {
				fmt.Fprintln(out, a.Description)
				fmt.Fprintln(out)
			}

			if p := ref.GetPackage(); p != nil {
				fmt.Fprintln(out, "Package: ", dropDataPrefix(p.Path))
			}
			if r := ref.GetRule(); r != nil {
				fmt.Fprintln(out, "Rule:    ", r.Head.Name)
			}
			if loc := ref.Location; loc != nil {
				fmt.Fprintln(out, "Location:", loc.String())
			}
			if a := ref.Annotations; a != nil {
				if len(a.Scope) > 0 {
					fmt.Fprintln(out, "Scope:", a.Scope)
				}
				if a.Entrypoint {
					fmt.Fprintln(out, "Entrypoint:", a.Entrypoint)
				}
			}
			fmt.Fprintln(out)

			if a := ref.Annotations; a != nil {
				if len(a.Organizations) > 0 {
					fmt.Fprintln(out, "Organizations:")
					l := make([]listEntry, 0, len(a.Organizations))
					for _, o := range a.Organizations {
						l = append(l, listEntry{"", removeNewLines(o)})
					}
					printList(out, l, "")
					fmt.Fprintln(out)
				}

				if len(a.Authors) > 0 {
					fmt.Fprintln(out, "Authors:")
					l := make([]listEntry, 0, len(a.Authors))
					for _, a := range a.Authors {
						l = append(l, listEntry{"", removeNewLines(a.String())})
					}
					printList(out, l, "")
					fmt.Fprintln(out)
				}

				if len(a.Schemas) > 0 {
					// NOTE(johanfylling): The Type Checker will MERGE all applicable schema annotations for a rule
					// into one list. Here, child nodes OVERRIDE parent nodes' schema annotations instead (default annot. behavior).
					// Should the former behavior be replicated here?
					fmt.Fprintln(out, "Schemas:")
					l := make([]listEntry, 0, len(a.Schemas))
					for _, s := range a.Schemas {
						le := listEntry{key: s.Path.String()}
						if len(s.Schema) > 0 {
							le.value = s.Schema.String()
						} else if s.Definition != nil {
							b, _ := json.Marshal(s.Definition)
							le.value = string(b)
						}
						l = append(l, le)
					}
					printList(out, l, ": ")
					fmt.Fprintln(out)
				}

				if len(a.RelatedResources) > 0 {
					fmt.Fprintln(out, "Related Resources:")
					l := make([]listEntry, 0, len(a.RelatedResources))
					for _, res := range a.RelatedResources {
						l = append(l, listEntry{removeNewLines(res.Ref.String()), res.Description})
					}
					printList(out, l, " ")
					fmt.Fprintln(out)
				}
				if len(a.Custom) > 0 {
					fmt.Fprintln(out, "Custom:")
					l := make([]listEntry, 0, len(a.Custom))
					for k, v := range a.Custom {
						b, _ := json.Marshal(v)
						l = append(l, listEntry{k, string(b)})
					}
					printList(out, l, ": ")
					fmt.Fprintln(out)
				}
			}
		}
	}

	return nil
}

type listEntry struct {
	key   string
	value string
}

func printList(out io.Writer, list []listEntry, separator string) {
	keyLength := 0
	for _, e := range list {
		l := len(e.key)
		if l > keyLength {
			keyLength = l
		}
	}
	for _, e := range list {
		line := fmt.Sprintf(" %s%s%s%s",
			e.key,
			separator,
			strings.Repeat(" ", keyLength-len(e.key)),
			e.value)
		fmt.Fprintln(out, truncateStr(line, pageWidth))
	}
}

func printTitle(out io.Writer, ref *ast.AnnotationsRef) {
	var title string
	if a := ref.Annotations; a != nil {
		t := strings.TrimSpace(a.Title)
		if len(t) > 0 {
			title = t
		}
	}

	if len(title) == 0 {
		title = dropDataPrefix(ref.Path).String()
	}

	fmt.Fprintf(out, "%s\n", title)

	var underline []byte
	for i := 0; i < len(title) && i < pageWidth; i++ {
		underline = append(underline, '=')
	}
	fmt.Fprintln(out, string(underline))
}

func generateTableWithKeys(writer io.Writer, keys ...string) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	aligns := []int{}
	hdrs := make([]string, 0, len(keys))
	for _, k := range keys {
		hdrs = append(hdrs, strings.Title(k)) //nolint:staticcheck // SA1019, no unicode
		aligns = append(aligns, tablewriter.ALIGN_LEFT)
	}
	table.SetHeader(hdrs)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetAutoMergeCells(true)
	table.SetColumnAlignment(aligns)
	table.SetRowLine(false)
	table.SetAutoWrapText(false)
	return table
}

func truncateTableStr(s string) string {
	return truncateStr(s, maxTableFieldLen)
}

func truncateStr(s string, maxLen int) string {
	if len(s) < maxLen {
		return s
	}
	return fmt.Sprintf("%v...", s[:maxLen-3])
}

func removeNewLines(s string) string {
	return strings.ReplaceAll(s, "\n", " ")
}

func truncateFileName(s string) string {
	if len(s) < maxTableFieldLen {
		return s
	}

	res, _ := iStrs.TruncateFilePaths(maxTableFieldLen, len(s), s)
	return res[s]
}

// dropDataPrefix drops the first component of the passed Ref
func dropDataPrefix(ref ast.Ref) ast.Ref {
	if len(ref) <= 1 {
		return ast.EmptyRef()
	}
	r := ref[1:].Copy()
	if s, ok := r[0].Value.(ast.String); ok {
		r[0].Value = ast.Var(s)
	}
	return r
}
