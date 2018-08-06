// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/context"
	"golang.org/x/perf/benchstat"
	"golang.org/x/perf/storage/benchfmt"
	"golang.org/x/perf/storage/query"
)

// A resultGroup holds a list of results and tracks the distinct labels found in that list.
type resultGroup struct {
	// The (partial) query string that resulted in this group.
	Q string
	// Raw list of results.
	results []*benchfmt.Result
	// LabelValues is the count of results found with each distinct (key, value) pair found in labels.
	// A value of "" counts results missing that key.
	LabelValues map[string]valueSet
}

// add adds res to the resultGroup.
func (g *resultGroup) add(res *benchfmt.Result) {
	g.results = append(g.results, res)
	if g.LabelValues == nil {
		g.LabelValues = make(map[string]valueSet)
	}
	for k, v := range res.Labels {
		if g.LabelValues[k] == nil {
			g.LabelValues[k] = make(valueSet)
			if len(g.results) > 1 {
				g.LabelValues[k][""] = len(g.results) - 1
			}
		}
		g.LabelValues[k][v]++
	}
	for k := range g.LabelValues {
		if res.Labels[k] == "" {
			g.LabelValues[k][""]++
		}
	}
}

// splitOn returns a new set of groups sharing a common value for key.
func (g *resultGroup) splitOn(key string) []*resultGroup {
	groups := make(map[string]*resultGroup)
	var values []string
	for _, res := range g.results {
		value := res.Labels[key]
		if groups[value] == nil {
			groups[value] = &resultGroup{Q: key + ":" + value}
			values = append(values, value)
		}
		groups[value].add(res)
	}

	sort.Strings(values)
	var out []*resultGroup
	for _, value := range values {
		out = append(out, groups[value])
	}
	return out
}

// valueSet is a set of values and the number of results with each value.
type valueSet map[string]int

// valueCount and byCount are used for sorting a valueSet
type valueCount struct {
	Value string
	Count int
}
type byCount []valueCount

func (s byCount) Len() int      { return len(s) }
func (s byCount) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s byCount) Less(i, j int) bool {
	if s[i].Count != s[j].Count {
		return s[i].Count > s[j].Count
	}
	return s[i].Value < s[j].Value
}

// TopN returns a slice containing n valueCount entries, and if any labels were omitted, an extra entry with value "…".
func (vs valueSet) TopN(n int) []valueCount {
	var s []valueCount
	var total int
	for v, count := range vs {
		s = append(s, valueCount{v, count})
		total += count
	}
	sort.Sort(byCount(s))
	out := s
	if len(out) > n {
		out = s[:n]
	}
	if len(out) < len(s) {
		var outTotal int
		for _, vc := range out {
			outTotal += vc.Count
		}
		out = append(out, valueCount{"…", total - outTotal})
	}
	return out
}

// addToQuery returns a new query string with add applied as a filter.
func addToQuery(query, add string) string {
	if strings.ContainsAny(add, " \t\\\"") {
		add = strings.Replace(add, `\`, `\\`, -1)
		add = strings.Replace(add, `"`, `\"`, -1)
		add = `"` + add + `"`
	}
	if strings.Contains(query, "|") {
		return add + " " + query
	}
	return add + " | " + query
}

// linkify returns a link related to the label's value. If no such link exists, it returns an empty string.
// For example, "cl: 1234" is linked to golang.org/cl/1234.
// string is used as the return type and not template.URL so that html/template will validate the scheme.
func linkify(labels benchfmt.Labels, label string) string {
	switch label {
	case "cl", "commit":
		return "https://golang.org/cl/" + template.URLQueryEscaper(labels[label])
	case "ps":
		// TODO(quentin): Figure out how to link to a particular patch set on Gerrit.
		return ""
	case "repo":
		return labels["repo"]
	case "try":
		// TODO(quentin): Return link to farmer once farmer has permalinks.
		return ""
	}
	return ""
}

// compare handles queries that require comparison of the groups in the query.
func (a *App) compare(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	q := r.Form.Get("q")

	tmpl, err := ioutil.ReadFile(filepath.Join(a.BaseDir, "template/compare.html"))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	t, err := template.New("main").Funcs(template.FuncMap{
		"addToQuery": addToQuery,
		"linkify":    linkify,
	}).Parse(string(tmpl))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	data := a.compareQuery(ctx, q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

type compareData struct {
	Q            string
	Error        string
	Benchstat    template.HTML
	Groups       []*resultGroup
	Labels       map[string]bool
	CommonLabels benchfmt.Labels
}

// queryKeys returns the keys that are exact-matched by q.
func queryKeys(q string) map[string]bool {
	out := make(map[string]bool)
	for _, part := range query.SplitWords(q) {
		// TODO(quentin): This func is shared with db.go; refactor?
		i := strings.IndexFunc(part, func(r rune) bool {
			return r == ':' || r == '>' || r == '<' || unicode.IsSpace(r) || unicode.IsUpper(r)
		})
		if i >= 0 && part[i] == ':' {
			out[part[:i]] = true
		}
	}
	return out
}

// elideKeyValues returns content, a benchmark format line, with the
// values of any keys in keys elided.
func elideKeyValues(content string, keys map[string]bool) string {
	var end string
	if i := strings.IndexFunc(content, unicode.IsSpace); i >= 0 {
		content, end = content[:i], content[i:]
	}
	// Check for gomaxprocs value
	if i := strings.LastIndex(content, "-"); i >= 0 {
		_, err := strconv.Atoi(content[i+1:])
		if err == nil {
			if keys["gomaxprocs"] {
				content, end = content[:i], "-*"+end
			} else {
				content, end = content[:i], content[i:]+end
			}
		}
	}
	parts := strings.Split(content, "/")
	for i, part := range parts {
		if equals := strings.Index(part, "="); equals >= 0 {
			if keys[part[:equals]] {
				parts[i] = part[:equals] + "=*"
			}
		} else if i == 0 {
			if keys["name"] {
				parts[i] = "Benchmark*"
			}
		} else if keys[fmt.Sprintf("sub%d", i)] {
			parts[i] = "*"
		}
	}
	return strings.Join(parts, "/") + end
}

// fetchCompareResults fetches the matching results for a given query string.
// The results will be grouped into one or more groups based on either the query string or heuristics.
func (a *App) fetchCompareResults(ctx context.Context, q string) ([]*resultGroup, error) {
	// Parse query
	prefix, queries := parseQueryString(q)

	// Send requests
	// TODO(quentin): Issue requests in parallel?
	var groups []*resultGroup
	var found int
	for _, qPart := range queries {
		keys := queryKeys(qPart)
		group := &resultGroup{Q: qPart}
		if prefix != "" {
			qPart = prefix + " " + qPart
		}
		res := a.StorageClient.Query(ctx, qPart)
		for res.Next() {
			result := res.Result()
			result.Content = elideKeyValues(result.Content, keys)
			group.add(result)
			found++
		}
		err := res.Err()
		res.Close()
		if err != nil {
			// TODO: If the query is invalid, surface that to the user.
			return nil, err
		}
		groups = append(groups, group)
	}

	if found == 0 {
		return nil, errors.New("no results matched the query string")
	}

	// Attempt to automatically split results.
	if len(groups) == 1 {
		group := groups[0]
		// Matching a single CL -> split by filename
		switch {
		case len(group.LabelValues["cl"]) == 1 && len(group.LabelValues["ps"]) == 1 && len(group.LabelValues["upload-file"]) > 1:
			groups = group.splitOn("upload-file")
		// Matching a single upload with multiple files -> split by file
		case len(group.LabelValues["upload"]) == 1 && len(group.LabelValues["upload-part"]) > 1:
			groups = group.splitOn("upload-part")
		}
	}

	return groups, nil
}

func (a *App) compareQuery(ctx context.Context, q string) *compareData {
	if len(q) == 0 {
		return &compareData{}
	}

	groups, err := a.fetchCompareResults(ctx, q)
	if err != nil {
		return &compareData{
			Q:     q,
			Error: err.Error(),
		}
	}

	var buf bytes.Buffer
	// Compute benchstat
	c := &benchstat.Collection{
		AddGeoMean: true,
		SplitBy:    nil,
	}
	for _, label := range []string{"buildlet", "pkg", "goos", "goarch"} {
		for _, g := range groups {
			if len(g.LabelValues[label]) > 1 {
				c.SplitBy = append(c.SplitBy, label)
				break
			}
		}
	}
	for _, g := range groups {
		c.AddResults(g.Q, g.results)
	}
	benchstat.FormatHTML(&buf, c.Tables())

	// Prepare struct for template.
	labels := make(map[string]bool)
	// commonLabels are the key: value of every label that has an
	// identical value on every result.
	commonLabels := make(benchfmt.Labels)
	// Scan the first group for common labels.
	for k, vs := range groups[0].LabelValues {
		if len(vs) == 1 {
			for v := range vs {
				commonLabels[k] = v
			}
		}
	}
	// Remove any labels not common in later groups.
	for _, g := range groups[1:] {
		for k, v := range commonLabels {
			if len(g.LabelValues[k]) != 1 || g.LabelValues[k][v] == 0 {
				delete(commonLabels, k)
			}
		}
	}
	// List all labels present and not in commonLabels.
	for _, g := range groups {
		for k := range g.LabelValues {
			if commonLabels[k] != "" {
				continue
			}
			labels[k] = true
		}
	}
	data := &compareData{
		Q:            q,
		Benchstat:    template.HTML(buf.String()),
		Groups:       groups,
		Labels:       labels,
		CommonLabels: commonLabels,
	}
	return data
}

// textCompare is called if benchsave is requesting a text-only analysis.
func (a *App) textCompare(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	q := r.Form.Get("q")

	groups, err := a.fetchCompareResults(ctx, q)
	if err != nil {
		// TODO(quentin): Should we serve this with a 500 or 404? This means the query was invalid or had no results.
		fmt.Fprintf(w, "unable to analyze results: %v", err)
	}

	// Compute benchstat
	c := new(benchstat.Collection)
	for _, g := range groups {
		c.AddResults(g.Q, g.results)
	}
	benchstat.FormatText(w, c.Tables())
}
