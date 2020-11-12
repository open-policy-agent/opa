// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package encoding

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/internal/compiler/wasm/opa"
)

func TestRoundtrip(t *testing.T) {

	bs, err := ioutil.ReadFile(filepath.Join("testdata", "test1.wasm"))
	if err != nil {
		t.Fatal(err)
	}

	module, err := ReadModule(bytes.NewBuffer(bs))
	if err != nil {
		t.Fatal(err)
	}

	entries, err := CodeEntries(module)
	if err != nil {
		t.Fatal(err)
	}

	for i, e := range entries {

		var buf3 bytes.Buffer

		if err := WriteCodeEntry(&buf3, e); err != nil {
			t.Fatal(err)
		}

		module.Code.Segments[i].Code = buf3.Bytes()
	}

	var buf2 bytes.Buffer

	if err := WriteModule(&buf2, module); err != nil {
		t.Fatal(err)
	}

	module2, err := ReadModule(&buf2)
	if err != nil {
		t.Fatal(err)
	}

	// TODO(tsandall): how to make this more debuggable
	if !reflect.DeepEqual(module, module2) {
		t.Fatal("modules are not equal")
	}

}

func TestRoundtripOPA(t *testing.T) {

	bs, err := opa.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	module, err := ReadModule(bytes.NewBuffer(bs))
	if err != nil {
		t.Fatal(err)
	}
	// When using a WASM module with or without debug, the custom sections differ.
	// Both variants have 'name' and 'producers'.
	customSections := map[string]int{}
	for _, s := range module.Customs {
		customSections[s.Name]++
	}
	if expected, actual := 1, customSections["name"]; expected != actual {
		t.Errorf("expected %d 'name' custom sections, found %d", expected, actual)
	}
	if expected, actual := 1, customSections["producers"]; expected != actual {
		t.Errorf("expected %d 'producers' custom sections, found %d", expected, actual)
	}

	// TODO(tsandall): when all instructions are handled by reader, add logic to
	// check code section contents.

	var buf2 bytes.Buffer
	if err := WriteModule(&buf2, module); err != nil {
		t.Fatal(err)
	}

	module2, err := ReadModule(&buf2)
	if err != nil {
		t.Fatal(err)
	}

	// TODO(tsandall): how to make this more debuggable
	if !reflect.DeepEqual(module, module2) {
		t.Fatal("modules are not equal")
	}

}
