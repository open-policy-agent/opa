// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package future

import (
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
)

// FilterFutureImports filters OUT any future imports from the passed slice of
// `*ast.Import`s.
func FilterFutureImports(imps []*ast.Import) []*ast.Import {
	return slices.DeleteFunc(slices.Clone(imps), isFutureKeywordImport)
}

// IsAllFutureKeywords returns true if the passed *ast.Import is `future.keywords`
func IsAllFutureKeywords(imp *ast.Import) bool {
	path := imp.Path.Value.(ast.Ref)
	return len(path) == 2 && path.HasPrefix(ast.FutureKeywordsRef)
}

// IsFutureKeyword returns true if the passed *ast.Import is `future.keywords.{kw}`
func IsFutureKeyword(imp *ast.Import, kw string) bool {
	path := imp.Path.Value.(ast.Ref)
	return len(path) == 3 && path.HasPrefix(ast.FutureKeywordsRef) && path[2].Equal(ast.InternedTerm(kw))
}

func WhichFutureKeyword(imp *ast.Import) (string, bool) {
	name := imp.Name().String()
	return name, imp.Alias == "" && IsFutureKeyword(imp, name)
}

func isFutureKeywordImport(imp *ast.Import) bool {
	path := imp.Path.Value.(ast.Ref)
	return len(path) > 0 && path.HasPrefix(ast.FutureKeywordsRef[:1])
}
