// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package presentation prints results of an expression evaluation in
// json and tabular formats.
package presentation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/format"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/profiler"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
)

// Output contains the result of evaluation to be presented.
type Output struct {
	Error       error                `json:"error,omitempty"`
	Result      rego.ResultSet       `json:"result,omitempty"`
	Partial     *rego.PartialQueries `json:"partial,omitempty"`
	Metrics     metrics.Metrics      `json:"metrics,omitempty"`
	Explanation []*topdown.Event     `json:"explanation,omitempty"`
	Profile     []profiler.ExprStats `json:"profile,omitempty"`
	limit       int
}

// WithLimit sets the output limit to set on stringified values.
func (e Output) WithLimit(n int) Output {
	e.limit = n
	return e
}

func (e Output) undefined() bool {
	return len(e.Result) == 0 && (e.Partial == nil || len(e.Partial.Queries) == 0)
}

// JSON writes x to w with indentation.
func JSON(w io.Writer, x interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(x)
}

// Bindings prints the bindings from r to w.
func Bindings(w io.Writer, r Output) error {
	if r.Error != nil {
		return prettyError(w, r.Error)
	}
	for _, rs := range r.Result {
		if err := JSON(w, rs.Bindings); err != nil {
			return err
		}
	}
	return nil
}

// Values prints the values from r to w.
func Values(w io.Writer, r Output) error {
	if r.Error != nil {
		return prettyError(w, r.Error)
	}
	for _, rs := range r.Result {
		line := make([]interface{}, len(rs.Expressions))
		for i := range line {
			line[i] = rs.Expressions[i].Value
		}
		if err := JSON(os.Stdout, line); err != nil {
			return err
		}
	}
	return nil
}

// Pretty prints all of r to w in a human-readable format.
func Pretty(w io.Writer, r Output) error {
	if len(r.Explanation) > 0 {
		if err := prettyExplanation(w, r.Explanation); err != nil {
			return err
		}
	}
	if r.Error != nil {
		if err := prettyError(w, r.Error); err != nil {
			return err
		}
	} else if r.undefined() {
		fmt.Fprintln(w, "undefined")
	} else if r.Result != nil {
		if err := prettyResult(w, r.Result, r.limit); err != nil {
			return err
		}
	} else if r.Partial != nil {
		if err := prettyPartial(w, r.Partial); err != nil {
			return err
		}
	}
	if r.Metrics != nil {
		if err := prettyMetrics(w, r.Metrics, r.limit); err != nil {
			return err
		}
	}
	if len(r.Profile) > 0 {
		if err := prettyProfile(w, r.Profile); err != nil {
			return err
		}
	}
	return nil
}

func prettyError(w io.Writer, err error) error {
	_, err = fmt.Fprintln(w, err)
	return err
}

func prettyResult(w io.Writer, rs rego.ResultSet, limit int) error {

	if len(rs) == 1 && len(rs[0].Bindings) == 0 {
		if len(rs[0].Expressions) == 1 || allBoolean(rs[0].Expressions) {
			return JSON(w, rs[0].Expressions[0].Value)
		}
	}

	keys := generateResultKeys(rs)
	tableBindings := generateTableBindings(w, keys, rs, limit)
	if tableBindings.NumLines() > 0 {
		tableBindings.Render()
	}

	return nil
}

func prettyPartial(w io.Writer, pq *rego.PartialQueries) error {

	table := tablewriter.NewWriter(w)
	table.SetRowLine(true)
	table.SetAutoWrapText(false)
	var maxWidth int

	for i := range pq.Queries {
		s, width, err := prettyASTNode(pq.Queries[i])
		if err != nil {
			return err
		}
		if width > maxWidth {
			maxWidth = width
		}
		table.Append([]string{fmt.Sprintf("Query %d", i+1), s})
	}

	for i := range pq.Support {
		s, width, err := prettyASTNode(pq.Support[i])
		if err != nil {
			return err
		}
		if width > maxWidth {
			maxWidth = width
		}
		table.Append([]string{fmt.Sprintf("Support %d", i+1), s})
	}

	table.SetColMinWidth(1, maxWidth)
	table.Render()

	return nil
}

func prettyASTNode(x interface{}) (string, int, error) {
	setLocationRecursive(x)
	bs, err := format.Ast(x)
	if err != nil {
		return "", 0, err
	}
	var maxLineWidth int
	s := strings.Trim(strings.Replace(string(bs), "\t", "  ", -1), "\n")
	for _, line := range strings.Split(s, "\n") {
		width := tablewriter.DisplayWidth(line)
		if width > maxLineWidth {
			maxLineWidth = width
		}
	}
	return s, maxLineWidth, nil
}

func prettyMetrics(w io.Writer, m metrics.Metrics, limit int) error {
	tableMetrics := generateTableMetrics(w)
	populateTableMetrics(m, tableMetrics, limit)
	if tableMetrics.NumLines() > 0 {
		tableMetrics.Render()
	}
	return nil
}

func prettyProfile(w io.Writer, profile []profiler.ExprStats) error {
	tableProfile := generateTableProfile(w)
	for _, rs := range profile {
		line := []string{}
		timeNs := time.Duration(rs.ExprTimeNs) * time.Nanosecond
		timeNsStr := timeNs.String()
		numEval := strconv.FormatInt(int64(rs.NumEval), 10)
		numRedo := strconv.FormatInt(int64(rs.NumRedo), 10)
		loc := rs.Location.String()
		line = append(line, timeNsStr, numEval, numRedo, loc)
		tableProfile.Append(line)
	}
	if tableProfile.NumLines() > 0 {
		tableProfile.Render()
	}
	return nil
}

func prettyExplanation(w io.Writer, explanation []*topdown.Event) error {
	topdown.PrettyTrace(w, explanation)
	return nil
}

func checkStrLimit(input string, limit int) string {
	if limit > 0 && len(input) > limit {
		input = input[:limit] + "..."
		return input
	}
	return input
}

func generateTableBindings(writer io.Writer, keys []resultKey, rs rego.ResultSet, prettyLimit int) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetAutoFormatHeaders(false)
	header := make([]string, len(keys))
	for i := range header {
		header[i] = keys[i].string()
	}
	table.SetHeader(header)
	alignment := make([]int, len(keys))
	for i := range header {
		alignment[i] = tablewriter.ALIGN_LEFT
	}
	table.SetColumnAlignment(alignment)

	for _, row := range rs {
		printPrettyRow(table, keys, row, prettyLimit)
	}
	return table
}

func printPrettyRow(table *tablewriter.Table, keys []resultKey, result rego.Result, prettyLimit int) {
	buf := []string{}
	for _, k := range keys {
		v, ok := k.selectVarValue(result)
		if ok {
			js, err := json.Marshal(v)
			if err != nil {
				buf = append(buf, err.Error())
			} else {
				s := checkStrLimit(string(js), prettyLimit)
				buf = append(buf, s)
			}
		}
	}
	table.Append(buf)
}

func generateTableMetrics(writer io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Metric", "Value"})
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT})
	return table
}

func generateTableProfile(writer io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Time", "Num Eval", "Num Redo", "Location"})
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT})
	return table
}

func populateTableMetrics(m metrics.Metrics, table *tablewriter.Table, prettyLimit int) {
	lines := [][]string{}
	for varName, varValueInterface := range m.All() {
		val, ok := varValueInterface.(map[string]interface{})
		if !ok {
			line := []string{}
			varValue := checkStrLimit(fmt.Sprintf("%v", varValueInterface), prettyLimit)
			line = append(line, varName, varValue)
			lines = append(lines, line)
		} else {
			for k, v := range val {
				line := []string{}
				newVarName := fmt.Sprintf("%v_%v", varName, k)
				value := checkStrLimit(fmt.Sprintf("%v", v), prettyLimit)
				line = append(line, newVarName, value)
				lines = append(lines, line)
			}
		}
	}
	sortMetricRows(lines)
	table.AppendBulk(lines)
}

// setLocationRecursive walks the AST nodes under x and sets the location field
// using the string representation of the node. The format package requires
// that all AST nodes have a Location set. If any of the nodes under x are
// missing a Location, the format package returns an error.
func setLocationRecursive(x interface{}) {
	vis := ast.NewGenericVisitor(func(x interface{}) bool {
		switch x := x.(type) {
		case *ast.Package:
			x.Location = setLocation(x)
		case *ast.Import:
			x.Location = setLocation(x)
		case *ast.Rule:
			x.Location = setLocation(x)
		case *ast.Head:
			x.Location = setLocation(x)
		case *ast.Expr:
			x.Location = setLocation(x)
		case *ast.Term:
			x.Location = setLocation(x)
		case *ast.Comment:
			x.Location = setLocation(x)
		}
		return false
	})
	ast.Walk(vis, x)
}

func setLocation(x interface{}) *ast.Location {
	return ast.NewLocation([]byte(fmt.Sprint(x)), "", 1, 1)
}

func sortMetricRows(data [][]string) {
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})
}

type resultKey struct {
	varName   string
	exprIndex int
	exprText  string
}

func resultKeyLess(a, b resultKey) bool {
	if a.varName != "" {
		if b.varName == "" {
			return true
		}
		return a.varName < b.varName
	}
	return a.exprIndex < b.exprIndex
}

func (rk resultKey) string() string {
	if rk.varName != "" {
		return rk.varName
	}
	return rk.exprText
}

func (rk resultKey) selectVarValue(result rego.Result) (interface{}, bool) {
	if rk.varName != "" {
		return result.Bindings[rk.varName], true
	}
	val := result.Expressions[rk.exprIndex].Value
	if _, ok := val.(bool); ok {
		return nil, false
	}
	return val, true
}

func generateResultKeys(rs rego.ResultSet) []resultKey {
	keys := []resultKey{}
	if len(rs) != 0 {
		for k := range rs[0].Bindings {
			keys = append(keys, resultKey{
				varName: k,
			})
		}

		for i, expr := range rs[0].Expressions {
			if _, ok := expr.Value.(bool); !ok {
				keys = append(keys, resultKey{
					exprIndex: i,
					exprText:  expr.Text,
				})
			}
		}

		sort.Slice(keys, func(i, j int) bool {
			return resultKeyLess(keys[i], keys[j])
		})
	}
	return keys
}

func allBoolean(ev []*rego.ExpressionValue) bool {
	for i := range ev {
		if _, ok := ev[i].Value.(bool); !ok {
			return false
		}
	}
	return true
}
