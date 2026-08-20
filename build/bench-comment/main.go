// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Command bench-comment runs benchlab comparisons for BENCH_PKGS between
// BEFORE_SHA and AFTER_SHA, then renders (and, given GH_TOKEN and COMMIT_SHA,
// posts) a markdown PR comment covering only the benchmarks that changed
// significantly, each linking to its historical chart on the benchmarks site.
package main

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	opaModulePath     = "github.com/open-policy-agent/opa"
	benchmarksBaseURL = "https://open-policy-agent.github.io/opa/"
	defaultThreshold  = 15.0
	narrowTableHeader = `benchmark \ host`
)

var nonAlnumRE = regexp.MustCompile(`[^A-Za-z0-9]`)

// errNoBenchData indicates a raw benchmark output file contained no
// benchmark results at all (e.g. a package matched no benchmarks).
var errNoBenchData = errors.New("no benchmark data")

type change struct {
	absPct   float64
	deltaStr string
	pkg      string
	name     string
	url      string
}

func main() {
	beforeSHA := requireEnv("BEFORE_SHA")
	afterSHA := requireEnv("AFTER_SHA")

	var pkgs []string
	if err := json.Unmarshal([]byte(requireEnv("BENCH_PKGS")), &pkgs); err != nil {
		log.Fatalf("invalid BENCH_PKGS: %v", err)
	}
	for _, pkg := range pkgs {
		if err := runBenchlab(beforeSHA, afterSHA, pkg); err != nil {
			log.Fatalf("benchlab %s: %v", pkg, err)
		}
	}

	threshold := defaultThreshold
	if v := os.Getenv("BENCH_THRESHOLD_PCT"); v != "" {
		t, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Fatalf("invalid BENCH_THRESHOLD_PCT %q: %v", v, err)
		}
		threshold = t
	}

	files, err := filepath.Glob(".benchlab/benchstat.*.txt")
	if err != nil {
		log.Fatal(err)
	}
	slices.Sort(files)

	var changes []change
	for _, f := range files {
		cs, err := parseBenchstatFile(f)
		if err != nil {
			log.Fatalf("parsing %s: %v", f, err)
		}
		changes = append(changes, cs...)
	}

	var significant []change
	for _, c := range changes {
		if c.absPct >= threshold {
			significant = append(significant, c)
		}
	}
	slices.SortFunc(significant, func(a, b change) int {
		return cmp.Compare(b.absPct, a.absPct)
	})

	body := render(beforeSHA, afterSHA, threshold, significant)

	ghToken := os.Getenv("GH_TOKEN")
	commitSHA := os.Getenv("COMMIT_SHA")
	if ghToken == "" || commitSHA == "" {
		fmt.Print(body)
		return
	}
	if err := postComment(commitSHA, body); err != nil {
		log.Fatal(err)
	}
}

func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("%s must be set", name)
	}
	return v
}

// runBenchlab runs a single benchlab comparison for pkg, writing its
// bench/benchstat output under .benchlab/ for parseBenchstatFile to pick up.
//
// One benchmark run per rep, rather than benchlab's default of five: benchstat
// treats every sample as independent, but samples from one process share a map
// hash seed, a heap layout and a set of interned globals, so a cluster of five
// tells us far less than its size suggests. Sampling from fifteen processes
// instead of three keeps the total sample count while letting per-process
// effects land in the within-group variance, where they belong.
func runBenchlab(before, after, pkg string) error {
	cmd := exec.Command("benchlab",
		"-commit", before+","+after,
		"-pkg", pkg,
		"-host", "local:tags=opa_wasm",
		"-reps", "15",
		"-count", "1",
		"-benchtime", "300ms",
		"-run", "^$",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// postComment finds the PR that merged as commitSHA and posts body as a
// comment on it, via the gh CLI (which picks up GH_TOKEN from the environment).
func postComment(commitSHA, body string) error {
	out, err := exec.Command("gh", "pr", "list",
		"--search", commitSHA,
		"--state", "merged",
		"--json", "number",
		"--jq", ".[0].number",
	).Output()
	if err != nil {
		return fmt.Errorf("looking up PR for %s: %w", commitSHA, err)
	}

	prNumber := strings.TrimSpace(string(out))
	if prNumber == "" || prNumber == "null" {
		fmt.Printf("Could not find originating PR for commit %s\n", commitSHA)
		return nil
	}

	cmd := exec.Command("gh", "pr", "comment", prNumber, "--body-file", "-")
	cmd.Stdin = strings.NewReader(body)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func render(before, after string, threshold float64, changes []change) string {
	if len(changes) == 0 {
		return fmt.Sprintf("Benchmark comparison `%s` → `%s`: no changes ≥%.0f%%.\n", before, after, threshold)
	}

	var b strings.Builder

	fmt.Fprintf(&b, "**Benchmark comparison:** `%s` → `%s`\n\n", before, after)
	b.WriteString("| Benchmark | Δ |\n")
	b.WriteString("| --- | --- |\n")
	for _, c := range changes {
		label := c.pkg + ": " + c.name
		direction := "slower"
		if strings.HasPrefix(c.deltaStr, "-") {
			direction = "faster"
		}
		fmt.Fprintf(&b, "| [`%s`](%s) | **%s** (%s) |\n", label, c.url, c.deltaStr, direction)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "_Only benchmarks changing by %.0f%% or more are shown; click a benchmark to see its historical trend._\n\n", threshold)
	b.WriteString("_This comment was automatically generated by the benchmarks workflow._\n")

	return b.String()
}

// parseBenchstatFile extracts the significant (i.e. non-"~") rows from the
// narrow "benchmark \ host" comparison table that benchstat prints for a
// two-commit comparison, resolving each row to the package and GOMAXPROCS
// suffix recorded in the raw benchmark output the table was built from.
func parseBenchstatFile(path string) ([]change, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return nil, errors.New("empty file")
	}

	srcPath, err := sourceFileFromHeader(scanner.Text())
	if err != nil {
		return nil, err
	}
	pkg, gomaxprocs, err := readBenchMeta(srcPath)
	if errors.Is(err, errNoBenchData) {
		return nil, nil // e.g. a package with no matching benchmarks produced no results
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", srcPath, err)
	}

	pkgDisplay := strings.Replace(pkg, opaModulePath, ".", 1)
	shortPkg := strings.TrimPrefix(pkgDisplay, "./")
	if shortPkg == "" {
		shortPkg = pkgDisplay
	}

	var changes []change
	inTable := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, narrowTableHeader) {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name, delta := fields[0], fields[1]
		if !strings.HasPrefix(delta, "+") && !strings.HasPrefix(delta, "-") {
			continue // e.g. "~" (not statistically significant) or the "vs base" header
		}
		pct, err := strconv.ParseFloat(strings.TrimSuffix(delta, "%"), 64)
		if err != nil {
			continue
		}

		// benchstat strips the leading "Benchmark" from names in its tables,
		// but the published chart pages are keyed by the full Go benchmark
		// name (including GOMAXPROCS suffix), so it must be added back.
		fullName := name + "-" + gomaxprocs
		slug := nonAlnumRE.ReplaceAllString(pkgDisplay+"_Benchmark"+fullName, "_")

		changes = append(changes, change{
			absPct:   math.Abs(pct),
			deltaStr: delta,
			pkg:      shortPkg,
			name:     fullName,
			url:      benchmarksBaseURL + "benchmarks." + slug + ".html",
		})
	}

	return changes, scanner.Err()
}

// sourceFileFromHeader extracts the raw benchmark output file (e.g.
// ".benchlab/bench.2026-06-24.2.txt") that benchstat was run against, from
// its "# benchstat -flags... <file>" comment header.
func sourceFileFromHeader(header string) (string, error) {
	fields := strings.Fields(header)
	for _, field := range slices.Backward(fields) {
		if strings.HasSuffix(field, ".txt") {
			return field, nil
		}
	}
	return "", fmt.Errorf("no source file found in header %q", header)
}

// readBenchMeta reads the package import path and GOMAXPROCS suffix (e.g.
// "16" in "BenchmarkFoo-16") recorded in a raw `go test -bench` output file.
func readBenchMeta(path string) (pkg, gomaxprocs string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if pkg == "" {
			if p, ok := strings.CutPrefix(line, "pkg: "); ok {
				pkg = p
			}
		}
		if gomaxprocs == "" {
			if n, ok := benchmarkGOMAXPROCS(line); ok {
				gomaxprocs = n
			}
		}
		if pkg != "" && gomaxprocs != "" {
			return pkg, gomaxprocs, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	if pkg == "" {
		return "", "", errNoBenchData
	}
	return "", "", errors.New("no benchmark result line found")
}

// benchmarkGOMAXPROCS extracts the trailing "-N" GOMAXPROCS suffix Go
// appends to benchmark names, from a `go test -bench` result line such as
// "BenchmarkFoo/bar-16    1000    123 ns/op".
func benchmarkGOMAXPROCS(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "Benchmark") {
		return "", false
	}
	name := fields[0]
	idx := strings.LastIndex(name, "-")
	if idx == -1 {
		return "", false
	}
	suffix := name[idx+1:]
	if _, err := strconv.Atoi(suffix); err != nil {
		return "", false
	}
	return suffix, true
}
