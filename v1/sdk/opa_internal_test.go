package sdk

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	sdktest "github.com/open-policy-agent/opa/v1/sdk/test"
)

func TestDefaultOptions(t *testing.T) {
	ctx := t.Context()
	server := sdktest.MustNewServer(
		sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"main.rego": `
package system

loaded := true
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

	opts := defaultOptions
	opts.Config = strings.NewReader(config)
	SetDefaultOptions(opts)

	t.Cleanup(func() { SetDefaultOptions(defaultOptions) })

	opa, err := New(ctx, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer opa.Stop(ctx)

	exp := true

	if result, err := opa.Decision(ctx, DecisionOptions{Path: "/system/loaded"}); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(result.Result, exp) {
		t.Fatalf("expected %v but got %v", exp, result.Result)
	}
}
