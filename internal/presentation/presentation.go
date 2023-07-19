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
	"github.com/open-policy-agent/opa/cover"
	"github.com/open-policy-agent/opa/format"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/profiler"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
)

// DefaultProfileSortOrder is the default ordering unless something is specified in the CLI
var DefaultProfileSortOrder = []string{"total_time_ns", "num_eval", "num_redo", "file", "line"}

// DepAnalysisOutput contains the result of dependency analysis to be presented.
type DepAnalysisOutput struct {
	Base    []ast.Ref `json:"base,omitempty"`
	Virtual []ast.Ref `json:"virtual,omitempty"`
}

// JSON outputs o to w as JSON.
func (o DepAnalysisOutput) JSON(w io.Writer) error {
	o.sort()
	return JSON(w, o)
}

// Pretty outputs o to w in a human-readable format.
func (o DepAnalysisOutput) Pretty(w io.Writer) error {

	var headers []string
	var rows [][]string

	// Fill two columns if results have base and virtual docs. Else fill one column.
	if len(o.Base) > 0 && len(o.Virtual) > 0 {
		maxLen := len(o.Base)
		if len(o.Virtual) > maxLen {
			maxLen = len(o.Virtual)
		}
		headers = []string{"Base Documents", "Virtual Documents"}
		rows = make([][]string, maxLen)
		for i := range rows {
			rows[i] = make([]string, 2)
			if i < len(o.Base) {
				rows[i][0] = o.Base[i].String()
			}
			if i < len(o.Virtual) {
				rows[i][1] = o.Virtual[i].String()
			}
		}
	} else if len(o.Base) > 0 {
		headers = []string{"Base Documents"}
		rows = make([][]string, len(o.Base))
		for i := range rows {
			rows[i] = []string{o.Base[i].String()}
		}
	} else if len(o.Virtual) > 0 {
		headers = []string{"Virtual Documents"}
		rows = make([][]string, len(o.Virtual))
		for i := range rows {
			rows[i] = []string{o.Virtual[i].String()}
		}
	}

	if len(rows) == 0 {
		return nil
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	for i := range rows {
		table.Append(rows[i])
	}

	table.Render()

	return nil
}

func (o DepAnalysisOutput) sort() {
	sort.Slice(o.Base, func(i, j int) bool {
		return o.Base[i].Compare(o.Base[j]) < 0
	})

	sort.Slice(o.Virtual, func(i, j int) bool {
		return o.Virtual[i].Compare(o.Virtual[j]) < 0
	})
}

// Output contains the result of evaluation to be presented.
type Output struct {
	Errors            OutputErrors                   `json:"errors,omitempty"`
	Result            rego.ResultSet                 `json:"result,omitempty"`
	Partial           *rego.PartialQueries           `json:"partial,omitempty"`
	Metrics           metrics.Metrics                `json:"metrics,omitempty"`
	AggregatedMetrics map[string]interface{}         `json:"aggregated_metrics,omitempty"`
	Explanation       []*topdown.Event               `json:"explanation,omitempty"`
	Profile           []profiler.ExprStats           `json:"profile,omitempty"`
	AggregatedProfile []profiler.ExprStatsAggregated `json:"aggregated_profile,omitempty"`
	Coverage          *cover.Report                  `json:"coverage,omitempty"`
	limit             int
}

// WithLimit sets the output limit to set on stringified values.
func (e Output) WithLimit(n int) Output {
	e.limit = n
	return e
}

func (e Output) undefined() bool {
	return len(e.Result) == 0 && (e.Partial == nil || len(e.Partial.Queries) == 0)
}

// NewOutputErrors creates a new slice of OutputError's based
// on the type of error passed in. Known structured types will
// be translated as appropriate, while unknown errors are
// placed into a structured format with their string value.
func NewOutputErrors(err error) []OutputError {
	var errs []OutputError
	if err != nil {
		// Handle known structured errors

		switch typedErr := err.(type) {
		case *ast.Error:
			oe := OutputError{
				Code:    typedErr.Code,
				Message: typedErr.Message,
				Details: typedErr.Details,
				err:     typedErr,
			}

			// TODO(patrick-east): Why does the JSON marshaller marshal
			// location as `null` when err.location == nil?!
			if typedErr.Location != nil {
				oe.Location = typedErr.Location
			}
			errs = []OutputError{oe}
		case *topdown.Error:
			errs = []OutputError{{
				Code:     typedErr.Code,
				Message:  typedErr.Message,
				Location: typedErr.Location,
				err:      typedErr,
			}}
		case *storage.Error:
			errs = []OutputError{{
				Code:    typedErr.Code,
				Message: typedErr.Message,
				err:     typedErr,
			}}

		// The cases below are wrappers for other errors, format errors
		// recursively on them.
		case ast.Errors:
			for _, e := range typedErr {
				if e != nil {
					errs = append(errs, NewOutputErrors(e)...)
				}
			}
		case rego.Errors:
			for _, e := range typedErr {
				if e != nil {
					errs = append(errs, NewOutputErrors(e)...)
				}
			}
		case loader.Errors:
			{
				for _, e := range typedErr {
					if e != nil {
						errs = append(errs, NewOutputErrors(e)...)
					}
				}
			}
		default:
			// Any errors which don't have a structure we know about
			// are converted to their string representation only.
			errs = []OutputError{{
				Message: err.Error(),
				err:     typedErr,
			}}
			if d, ok := err.(rego.ErrorDetails); ok {
				details := strings.Join(d.Lines(), "\n")
				errs[0].Details = details
			}
		}
	}
	return errs
}

// OutputErrors is a list of errors encountered
// which are to presented.
type OutputErrors []OutputError

func (e OutputErrors) Error() string {
	if len(e) == 0 {
		return "no error(s)"
	}

	var prefix string
	if len(e) == 1 {
		prefix = "1 error occurred: "
	} else {
		prefix = fmt.Sprintf("%d errors occurred:\n", len(e))
	}

	// We preallocate for at least the minimum number of strings.
	s := make([]string, 0, len(e))
	for _, err := range e {
		s = append(s, err.Error())
		if l, ok := err.Details.(string); ok {
			s = append(s, l)
		}
	}

	return prefix + strings.Join(s, "\n")
}

// OutputError provides a common structure for all OPA
// library errors so that the JSON output given by the
// presentation package is consistent and parsable.
type OutputError struct {
	Message  string      `json:"message"`
	Code     string      `json:"code,omitempty"`
	Location interface{} `json:"location,omitempty"`
	Details  interface{} `json:"details,omitempty"`
	err      error
}

func (j OutputError) Error() string {
	return j.err.Error()
}

// JSON writes x to w with indentation.
func JSON(w io.Writer, x interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(x)
}

// Bindings prints the bindings from r to w.
func Bindings(w io.Writer, r Output) error {
	if r.Errors != nil {
		return prettyError(w, r.Errors)
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
	if r.Errors != nil {
		return prettyError(w, r.Errors)
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
	if r.Errors != nil {
		if err := prettyError(w, r.Errors); err != nil {
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
	if len(r.AggregatedMetrics) > 0 {
		if err := prettyAggregatedMetrics(w, r.AggregatedMetrics, r.limit); err != nil {
			return err
		}
	}
	if len(r.AggregatedProfile) > 0 {
		if err := prettyAggregatedProfile(w, r.AggregatedProfile); err != nil {
			return err
		}
	}
	if r.Coverage != nil {
		if err := prettyCoverage(w, r.Coverage); err != nil {
			return err
		}
	}
	return nil
}

// Source prints partial evaluation results in r to w in a source file friendly
// format.
func Source(w io.Writer, r Output) error {

	if r.Errors != nil {
		return prettyError(w, r.Errors)
	}

	for i := range r.Partial.Queries {
		fmt.Fprintf(w, "# Query %d\n", i+1)
		bs, err := format.AstWithOpts(r.Partial.Queries[i], format.Opts{IgnoreLocations: true})
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(bs))
	}

	for i := range r.Partial.Support {
		fmt.Fprintf(w, "# Module %d\n", i+1)
		bs, err := format.AstWithOpts(r.Partial.Support[i], format.Opts{IgnoreLocations: true})
		if err != nil {
			return err
		}
		fmt.Fprint(w, string(bs))
	}

	return nil
}

// Raw prints the values from r to w.  Each result is written on a separate
// line, and the expressions are separated by spaces.  If the values are
// strings, they are written directly rather than formatted as compact
// JSON strings.  This output format makes OPA useful in a scripting context.
func Raw(w io.Writer, r Output) error {
	if r.Errors != nil {
		return prettyError(w, r.Errors)
	}

	for _, rs := range r.Result {
		for i, expr := range rs.Expressions {
			if str, ok := expr.Value.(string); ok {
				fmt.Fprint(w, str)
			} else {
				bytes, err := json.Marshal(expr.Value)
				if err != nil {
					return err
				}

				fmt.Fprint(w, string(bytes))
			}

			if i+1 >= len(rs.Expressions) {
				fmt.Fprintln(w, "")
			} else {
				fmt.Fprint(w, " ")
			}
		}
	}

	return nil
}

func Discard(w io.Writer, x interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	field, ok := x.(Output)
	if !ok {
		return fmt.Errorf("error in converting interface to type Output")
	}
	bs, err := json.Marshal(field)
	if err != nil {
		return err
	}
	var rawData map[string]interface{}
	err = json.Unmarshal(bs, &rawData)
	if err != nil {
		return err
	}
	if rawData["result"] != nil {
		rawData["result"] = "discarded"
	}
	return encoder.Encode(rawData)
}

func prettyError(w io.Writer, errs OutputErrors) error {
	_, err := fmt.Fprintln(w, errs)
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

// prettyASTNode is used for pretty-printing the result of partial eval
func prettyASTNode(x interface{}) (string, int, error) {
	bs, err := format.AstWithOpts(x, format.Opts{IgnoreLocations: true})
	if err != nil {
		return "", 0, fmt.Errorf("format error: %w", err)
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

var statKeys = []string{"min", "max", "mean", "90%", "99%"}

func prettyAggregatedMetrics(w io.Writer, ms map[string]interface{}, limit int) error {
	keys := []string{"metric"}
	tableMetrics := generateTableWithKeys(w, append(keys, statKeys...)...)
	populateTableAggregatedMetrics(ms, tableMetrics, limit)
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
		numGenExpr := strconv.FormatInt(int64(rs.NumGenExpr), 10)
		loc := rs.Location.String()
		line = append(line, timeNsStr, numEval, numRedo, numGenExpr, loc)
		tableProfile.Append(line)
	}
	if tableProfile.NumLines() > 0 {
		tableProfile.Render()
	}
	return nil
}

func prettyAggregatedProfile(w io.Writer, profile []profiler.ExprStatsAggregated) error {
	tableProfile := generateTableWithKeys(w, append(statKeys, "num eval", "num redo", "num gen expr", "location")...)
	for _, rs := range profile {
		line := []string{}
		for _, k := range statKeys {
			v := rs.ExprTimeNsStats.(map[string]interface{})[k]
			if f, ok := v.(float64); ok {
				line = append(line, time.Duration(f).String())
			} else if i, ok := v.(int64); ok {
				line = append(line, time.Duration(i).String())
			}
		}
		numEval := strconv.FormatInt(int64(rs.NumEval), 10)
		numRedo := strconv.FormatInt(int64(rs.NumRedo), 10)
		numGenExpr := strconv.FormatInt(int64(rs.NumGenExpr), 10)
		loc := rs.Location.String()
		line = append(line, numEval, numRedo, numGenExpr, loc)
		tableProfile.Append(line)
	}
	if tableProfile.NumLines() > 0 {
		tableProfile.Render()
	}
	return nil
}

func prettyExplanation(w io.Writer, explanation []*topdown.Event) error {
	topdown.PrettyTraceWithLocation(w, explanation)
	return nil
}

func prettyCoverage(w io.Writer, report *cover.Report) error {
	table := tablewriter.NewWriter(w)
	table.Append([]string{"Overall Coverage", fmt.Sprintf("%.02f", report.Coverage)})
	table.Render()
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
		v := k.selectVarValue(result)
		js, err := json.Marshal(v)
		if err != nil {
			buf = append(buf, err.Error())
		} else {
			s := checkStrLimit(string(js), prettyLimit)
			buf = append(buf, s)
		}
	}
	table.Append(buf)
}

func generateTableMetrics(writer io.Writer) *tablewriter.Table {
	return generateTableWithKeys(writer, "Metric", "Value")
}

func generateTableWithKeys(writer io.Writer, keys ...string) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	aligns := make([]int, 0, len(keys))
	hdrs := make([]string, 0, len(keys))
	for _, k := range keys {
		hdrs = append(hdrs, strings.Title(k)) //nolint:staticcheck // SA1019, no unicode here
		aligns = append(aligns, tablewriter.ALIGN_LEFT)
	}
	table.SetHeader(hdrs)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetColumnAlignment(aligns)
	return table
}

func generateTableProfile(writer io.Writer) *tablewriter.Table {
	return generateTableWithKeys(writer, "Time", "Num Eval", "Num Redo", "Num Gen Expr", "Location")
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

func populateTableAggregatedMetrics(ms map[string]interface{}, table *tablewriter.Table, prettyLimit int) {
	lines := [][]string{}
	for name, vals := range ms {
		line := []string{name}
		vs := vals.(map[string]interface{})
		for _, k := range statKeys {
			line = append(line, checkStrLimit(fmt.Sprintf("%v", vs[k]), prettyLimit))
		}
		lines = append(lines, line)
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

func (rk resultKey) selectVarValue(result rego.Result) interface{} {
	if rk.varName != "" {
		return result.Bindings[rk.varName]
	}
	return result.Expressions[rk.exprIndex].Value
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
			if _, ok := expr.Value.(bool); !ok || len(rs[0].Bindings) == 0 {
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
