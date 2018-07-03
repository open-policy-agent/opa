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
	"sort"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/open-policy-agent/opa/profiler"
	"github.com/open-policy-agent/opa/rego"
)

// EvalResult holds the results from evaluation, profiling and metrics.
type EvalResult struct {
	Result      rego.ResultSet         `json:"result,omitempty"`
	Explanation []string               `json:"explanation,omitempty"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
	Profile     []profiler.ExprStats   `json:"profile,omitempty"`
}

// PrintJSON prints indented json output.
func PrintJSON(writer io.Writer, x interface{}) error {
	buf, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(writer, string(buf))
	return nil
}

// PrintPretty prints expressions, bindings, metrics and profiling output
// in a tabular format.
func PrintPretty(writer io.Writer, result EvalResult, prettyLimit int) {

	// Bindings Table
	PrintPrettyBinding(writer, result, prettyLimit)

	// Metrics Table
	PrintPrettyMetrics(writer, result, prettyLimit)

	// Profile Table
	PrintPrettyProfile(writer, result)
}

// PrintPrettyBinding prints bindings in a tabular format
func PrintPrettyBinding(writer io.Writer, result EvalResult, prettyLimit int) {
	keys := generateResultKeys(result.Result)
	tableBindings := generateTableBindings(writer, keys, result, prettyLimit)

	if tableBindings.NumLines() > 0 {
		fmt.Println()
		tableBindings.Render()
	}
}

// PrintPrettyMetrics prints metrics in a tabular format
func PrintPrettyMetrics(writer io.Writer, result EvalResult, prettyLimit int) {
	tableMetrics := generateTableMetrics(writer)
	populateTableMetrics(result.Metrics, tableMetrics, prettyLimit)
	if tableMetrics.NumLines() > 0 {
		fmt.Println()
		tableMetrics.Render()
	}
}

// PrintPrettyProfile prints profiling stats in a tabular format
func PrintPrettyProfile(writer io.Writer, result EvalResult) {
	tableProfile := generateTableProfile(writer)
	for _, rs := range result.Profile {
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
		fmt.Println()
		tableProfile.Render()
	}
}

func checkStrLimit(input string, limit int) string {
	if limit > 0 && len(input) > limit {
		input = input[:limit] + "..."
		return input
	}
	return input
}

func generateTableBindings(writer io.Writer, keys []resultKey, result EvalResult, prettyLimit int) *tablewriter.Table {
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

	for _, row := range result.Result {
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
	table.SetHeader([]string{"Name", "Value"})
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

func populateTableMetrics(data map[string]interface{}, table *tablewriter.Table, prettyLimit int) {
	lines := [][]string{}
	for varName, varValueInterface := range data {
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
