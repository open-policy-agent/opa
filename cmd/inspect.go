// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter/tw"

	"github.com/open-policy-agent/opa/cmd/formats"
	"github.com/open-policy-agent/opa/cmd/internal/env"
	ib "github.com/open-policy-agent/opa/internal/bundle/inspect"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	iStrs "github.com/open-policy-agent/opa/internal/strings"
	"github.com/open-policy-agent/opa/v1/ast"
	astJson "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/util"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	maxTableFieldLen = 50
	pageWidth        = 80
)

type inspectCommandParams struct {
	outputFormat    *util.EnumFlag
	listAnnotations bool
	v0Compatible    bool
	v1Compatible    bool
}

func (p *inspectCommandParams) regoVersion() ast.RegoVersion {
	if p.v0Compatible {
		return ast.RegoV0
	}
	if p.v1Compatible {
		return ast.RegoV1
	}
	return ast.DefaultRegoVersion
}

func newInspectCommandParams() inspectCommandParams {
	return inspectCommandParams{
		outputFormat:    formats.Flag(formats.Pretty, formats.JSON),
		listAnnotations: false,
	}
}

func initInspect(root *cobra.Command, brand string) {
	executable := root.Name()

	params := newInspectCommandParams()

	inspectCommand := &cobra.Command{
		Use:   "inspect <path> [<path> [...]]",
		Short: `Inspect ` + brand + ` bundle(s)`,
		Long: `Inspect ` + brand + ` bundle(s).

The 'inspect' command provides a summary of the contents in ` + brand + ` bundle(s) or a single Rego file.
Bundles are gzipped tarballs containing policies and data. The 'inspect' command reads bundle(s) and lists
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
    $ ` + executable + ` inspect bundle.tar.gz

You can provide exactly one ` + brand + ` bundle, to a bundle directory, or direct path to a Rego file to the 'inspect'
command on the command-line. If you provide a path referring to a directory, the 'inspect' command will load that path as
a bundle and summarize its structure and contents.  If you provide a path referring to a Rego file, the 'inspect' command
will load that file and summarize its structure and contents.
`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateInspectParams(&params, args); err != nil {
				return err
			}
			return env.CmdFlags.CheckEnvironmentVariables(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			if err := doInspect(params, args[0], os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				return err
			}
			return nil
		},
	}

	addOutputFormat(inspectCommand.Flags(), params.outputFormat)
	addListAnnotations(inspectCommand.Flags(), &params.listAnnotations)
	addV0CompatibleFlag(inspectCommand.Flags(), &params.v0Compatible, false)
	addV1CompatibleFlag(inspectCommand.Flags(), &params.v1Compatible, false)
	root.AddCommand(inspectCommand)
}

func doInspect(params inspectCommandParams, path string, out io.Writer) error {
	info, err := ib.FileForRegoVersion(params.regoVersion(), path, params.listAnnotations)
	if err != nil {
		return err
	}

	switch params.outputFormat.String() {
	case formats.JSON:
		astJson.SetOptions(astJson.Options{
			MarshalOptions: astJson.MarshalOptions{
				IncludeLocation: astJson.NodeToggle{
					// Annotation location data is only included if includeAnnotations is set
					AnnotationsRef: params.listAnnotations,
				},
			},
		})
		defer astJson.SetOptions(astJson.Defaults())

		return pr.JSON(out, info)

	default:
		if hasManifest(info) {
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

func hasManifest(info *ib.Info) bool {
	if info.Manifest == nil {
		return false
	}
	return info.Manifest.Revision != "" || len(*info.Manifest.Roots) != 0 || len(info.Manifest.Metadata) != 0 ||
		info.Manifest.RegoVersion != nil
}

func validateInspectParams(p *inspectCommandParams, args []string) error {
	if len(args) != 1 {
		return errors.New("specify exactly one OPA bundle or path")
	}

	of := p.outputFormat.String()
	if of == formats.JSON || of == formats.Pretty {
		return nil
	}
	return errors.New("invalid output format for inspect command")
}

func populateManifest(out io.Writer, m *bundle.Manifest) error {
	t := generateTableWithKeys(out, "field", "value")
	var lines [][]string

	if m.RegoVersion != nil {
		lines = append(lines, []string{"Rego Version", truncateTableStr(strconv.Itoa(*m.RegoVersion))})
	}

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

	if len(lines) > 0 {
		if err := t.Bulk(lines); err != nil {
			return err
		}
		fmt.Fprintln(out, "MANIFEST:")
		if err := t.Render(); err != nil {
			return err
		}
	}

	return nil
}

func populateNamespaces(out io.Writer, n map[string][]string) error {
	t := generateTableWithKeys(out, "namespace", "file")
	// only auto-merge the namespace column
	t = t.Options(tablewriter.WithConfig(tablewriter.NewConfigBuilder().
		ForColumn(0).Build().Row().Formatting().WithMergeMode(tw.MergeVertical). // vertical merge column 0.
		Build().Build().Build(),                                                 //  build the table config.
	))

	var lines [][]string

	for _, k := range util.KeysSorted(n) {
		for _, file := range n[k] {
			lines = append(lines, []string{k, truncateFileName(file)})
		}
	}

	if err := t.Bulk(lines); err != nil {
		return err
	}
	if len(lines) > 0 {
		fmt.Fprintln(out, "NAMESPACES:")
		if err := t.Render(); err != nil {
			return err
		}
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
				fmt.Fprintln(out, "Rule:    ", r.Head.Ref().String())
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
		var line string
		if len(e.value) > 0 {
			line = fmt.Sprintf(" %s%s%s%s",
				e.key,
				separator,
				strings.Repeat(" ", keyLength-len(e.key)),
				e.value)
		} else {
			line = fmt.Sprintf(" %v", e.key)
		}
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

	fmt.Fprintf(out, "%s\n%s\n", title, strings.Repeat("=", min(len(title), pageWidth)))
}

func generateTableWithKeys(writer io.Writer, keys ...string) *tablewriter.Table {
	hdrs := make([]any, len(keys))
	for i, k := range keys {
		hdrs[i] = pr.TitleCase.String(k)
	}

	t := tablewriter.NewTable(
		writer,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{AutoFormat: tw.On},
				Alignment:  tw.CellAlignment{Global: tw.AlignCenter},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
				Formatting: tw.CellFormatting{
					AutoWrap:  tw.WrapNone,
					MergeMode: tw.MergeBoth,
				},
			},
		}),
		tablewriter.WithTrimLine(tw.Off),
	)

	t.Header(hdrs...)
	return t
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
