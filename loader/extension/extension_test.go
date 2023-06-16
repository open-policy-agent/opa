// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package extension_test

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/loader/extension"
	"github.com/open-policy-agent/opa/util"
)

func TestLoaderExtensionUnmarshal(t *testing.T) {
	sentinelErr := fmt.Errorf("test handler called")
	extension.RegisterExtension(".json", func([]byte, any) error {
		return sentinelErr
	})
	defer extension.RegisterExtension(".json", nil)

	bs := make([]byte, 128)
	_, err := rand.Read(bs)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := util.Unmarshal(bs, &v); err != sentinelErr {
		t.Error(err)
	}
}

func TestLoaderExtensionBundle(t *testing.T) {
	data := map[string]any{"foo": "bar"}
	extension.RegisterExtension(".json", func(_ []byte, x any) error {
		*(x.(*any)) = data
		return nil
	})
	defer extension.RegisterExtension(".json", nil)

	fs := fstest.MapFS{
		"data.json": {},
	}
	ldr := loader.NewFileLoader().WithFS(fs)
	res, err := ldr.All([]string{"."})
	if err != nil {
		t.Error(err)
	}
	if exp, act := data, res.Documents; !reflect.DeepEqual(exp, act) {
		t.Errorf("expected %v, got %v", exp, act)
	}
}
