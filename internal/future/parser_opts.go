// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package future

import (
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/v1/ast"
)

// ParserOptionsFromFutureImports transforms a slice of `ast.Import`s into the
// `ast.ParserOptions` that can be used to parse a statement according to the
// included "future.keywords" and "future.keywords.xyz" imports.
func ParserOptionsFromFutureImports(imports []*ast.Import) (ast.ParserOptions, error) {
	popts := ast.ParserOptions{}
	for _, imp := range imports {
		path := imp.Path.Value.(ast.Ref)
		if !ast.FutureRootDocument.Equal(path[0]) {
			continue
		}
		if len(path) >= 2 {
			if !path.HasPrefix(ast.FutureKeywordsRef) {
				return popts, fmt.Errorf("unknown future import: %v", imp)
			}
			if len(path) == 2 {
				// retun, one "future.keywords" import means we can disregard any others
				return ast.ParserOptions{AllFutureKeywords: true}, nil
			}
		}
		if len(path) == 3 {
			if imp.Alias != "" {
				return popts, errors.New("alias not supported")
			}
			popts.FutureKeywords = append(popts.FutureKeywords, string(path[2].Value.(ast.String)))
		}
	}
	return popts, nil
}

// ParserOptionsWithRegoVersionImports raises popts.RegoVersion to the version implied by any
// "rego.v1" and "rego.v2" imports. The version is never lowered: "rego.v1" leaves a v2 version as is.
func ParserOptionsWithRegoVersionImports(imports []*ast.Import, popts ast.ParserOptions) ast.ParserOptions {
	for _, imp := range imports {
		var v ast.RegoVersion
		switch {
		case ast.RegoV1CompatibleRef.Equal(imp.Path.Value):
			v = ast.RegoV1
		case ast.RegoV2CompatibleRef.Equal(imp.Path.Value):
			v = ast.RegoV2
		default:
			continue
		}
		if span, err := ast.RegoVersionSpanOf(popts.EffectiveRegoVersion()).RaisedTo(v); err == nil {
			popts.RegoVersion = span.Max()
		}
	}
	return popts
}
