// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"
)

func TestRegoParseModule(t *testing.T) {

	data := map[string]interface{}{
		"ok": `package foo.bar

		import data.a

		p { a = true }`,
		"err": `package foo.`,
	}

	runTopDownTestCase(t, data, "ok", []string{
		`p = x { rego.parse_module("x.rego", data.ok, module); x = module["package"].path[1].value }`}, `"foo"`)

	runTopDownTestCase(t, data, "error", []string{
		`p = x { rego.parse_module("x.rego", data.err, x) }`}, fmt.Errorf("rego_parse_error: no match found"))

}
