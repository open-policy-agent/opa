// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package conformance contains what OPA's YAML conformance corpora have in
// common: the diagnostic model they assert against, and the loader that reads a
// corpus from a directory tree.
package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"
)

// DefaultModuleName is the name given to the first module of a case, and the
// module errors are reported against unless stated otherwise.
const DefaultModuleName = "test-0.rego"

// ModuleName returns the name given to the i-th module of a case.
func ModuleName(i int) string {
	return fmt.Sprintf("test-%d.rego", i)
}

// Error is one expected diagnostic. Row and Col are 1-based positions in the
// module named by Module; Message is the error sentence, without the position
// and code an implementation may prefix it with when rendering.
type Error struct {
	Module  string `json:"module,omitempty"  yaml:"module,omitempty"` // module the error is reported against, defaults to test-0.rego
	Code    string `json:"code"              yaml:"code"`
	Row     int    `json:"row"               yaml:"row"`
	Col     int    `json:"col,omitempty"     yaml:"col,omitempty"` // asserted when non-zero
	Message string `json:"message"           yaml:"message"`
}

func (e Error) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", e.ModuleOrDefault(), e.Row, e.Col, e.Code, e.Message)
}

// ModuleOrDefault returns the module the error is reported against.
func (e Error) ModuleOrDefault() string {
	if e.Module == "" {
		return DefaultModuleName
	}
	return e.Module
}

// MatchErrors pairs every expected diagnostic with a distinct reported one, and
// returns those that could not be paired. Diagnostics are compared as a set: the
// order an implementation reports them in is not part of the contract. A zero Col
// in want matches any column. unexpected is populated only when exhaustive is
// set, i.e. when want is required to be the complete set rather than a subset.
func MatchErrors(want, got []Error, exhaustive bool) (missing, unexpected []Error) {
	matched := make([]bool, len(got))

	for _, w := range want {
		found := false
		for i, g := range got {
			if matched[i] || !errorMatches(w, g) {
				continue
			}
			matched[i] = true
			found = true
			break
		}
		if !found {
			missing = append(missing, w)
		}
	}

	if exhaustive {
		for i, g := range got {
			if !matched[i] {
				unexpected = append(unexpected, g)
			}
		}
	}

	return missing, unexpected
}

func errorMatches(want, got Error) bool {
	return want.ModuleOrDefault() == got.ModuleOrDefault() &&
		want.Code == got.Code &&
		want.Row == got.Row &&
		(want.Col == 0 || want.Col == got.Col) &&
		want.Message == got.Message
}

// CheckTrailingWhitespace rejects Rego that carries trailing whitespace on a
// line. A YAML emitter will not write a block scalar for such a value, so a
// generator would have to render the policy as a single escaped line, and
// nothing these corpora can express depends on that whitespace.
func CheckTrailingWhitespace(field, rego string) error {
	for i, line := range strings.Split(rego, "\n") {
		if strings.TrimRight(line, " \t") != line {
			return fmt.Errorf("%q line %d has trailing whitespace", field, i+1)
		}
	}
	return nil
}

// Case is implemented by the case type of a corpus.
type Case[T any] interface {
	// Name returns the globally unique note identifying the case.
	Name() string
	// WithFilename returns a copy of the case stamped with the file it was loaded from.
	WithFilename(string) T
}

// Set represents a collection of test cases.
type Set[T Case[T]] struct {
	Cases []T `json:"cases" yaml:"cases"`
}

// Sorted returns a copy of s with its cases ordered by name.
func (s Set[T]) Sorted() Set[T] {
	cpy := make([]T, len(s.Cases))
	copy(cpy, s.Cases)
	slices.SortFunc(cpy, func(a, b T) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return Set[T]{Cases: cpy}
}

// Load returns the set of test cases in the directory tree rooted at path.
func Load[T Case[T]](path string) (Set[T], error) {
	return load[T](os.DirFS(path), ".", func(p string) string {
		return filepath.Join(path, filepath.FromSlash(p))
	})
}

// MustLoad returns the set of test cases in the directory tree rooted at path,
// or panics if an error occurs.
func MustLoad[T Case[T]](path string) Set[T] {
	result, err := Load[T](path)
	if err != nil {
		panic(err)
	}
	return result
}

// LoadFS returns the set of test cases in the directory tree rooted at root in
// fsys, for corpora consumed through their embedded copy.
func LoadFS[T Case[T]](fsys fs.FS, root string) (Set[T], error) {
	return load[T](fsys, root, func(p string) string { return p })
}

func load[T Case[T]](fsys fs.FS, root string, filename func(string) string) (Set[T], error) {
	result := Set[T]{}

	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
			return nil
		}

		bs, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("%s: %w", filename(path), err)
		}

		var x Set[T]
		if err := Unmarshal(bs, &x); err != nil {
			return fmt.Errorf("%s: %w", filename(path), err)
		}

		for i := range x.Cases {
			x.Cases[i] = x.Cases[i].WithFilename(filename(path))
		}

		result.Cases = append(result.Cases, x.Cases...)
		return nil
	})

	return result, err
}

// Unmarshal decodes a parser or compiler corpus file, rejecting a field the case
// type does not declare. The corpora are generated, so an unknown field is debris
// rather than something to skip over.
func Unmarshal(bs []byte, v any) error {
	if json.Valid(bs) {
		return decodeStrict(bs, v)
	}

	nbs, err := yaml.YAMLToJSON(bs)
	if err != nil {
		return err
	}
	return decodeStrict(nbs, v)
}

func decodeStrict(bs []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(bs))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	return decoder.Decode(v)
}
