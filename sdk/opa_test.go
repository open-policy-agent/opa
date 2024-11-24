// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/sdk"
	sdktest "github.com/open-policy-agent/opa/v1/sdk/test"
)

func TestDefaultRegoVersion(t *testing.T) {

	ctx := context.Background()

	server := sdktest.MustNewServer(
		sdktest.RawBundles(true),
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			// v0 module
			"main.rego": `
package system

main {
	p[_] == "a"
}

p[x] {
	x = "a"
}

str = "foo"

loopback = input
`,
		}),
	)

	defer server.Stop()

	config := fmt.Sprintf(`{
		"services": {
			"test": {
				"url": %q
			}
		},
		"bundles": {
			"test": {
				"resource": "/bundles/bundle.tar.gz"
			}
		}
	}`, server.URL())

	opa, err := sdk.New(ctx, sdk.Options{
		Config: strings.NewReader(config),
	})
	if err != nil {
		t.Fatal(err)
	}

	defer opa.Stop(ctx)

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(bool); !ok || !decision {
		t.Fatal("expected true but got:", decision, ok)
	}

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{Path: "/system/str"}); err != nil {
		t.Fatal(err)
	} else if decision, ok := result.Result.(string); !ok || decision != "foo" {
		t.Fatal(`expected "foo" but got:`, decision)
	}

	exp := map[string]interface{}{"foo": "bar"}

	if result, err := opa.Decision(ctx, sdk.DecisionOptions{Path: "/system/loopback", Input: map[string]interface{}{"foo": "bar"}}); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(result.Result, exp) {
		t.Fatalf("expected %v but got %v", exp, result.Result)
	}
}
