package gojsonschema

import (
	"encoding/json"
	"testing"
)

func TestUUIDFormatCheckerIsFormat(t *testing.T) {
	checker := UUIDFormatChecker{}

	if !checker.IsFormat("01234567-89ab-cdef-0123-456789abcdef") {
		t.Error("Expected true, got false")
	}

	if !checker.IsFormat("f1234567-89ab-cdef-0123-456789abcdef") {
		t.Error("Expected true, got false")
	}

	if !checker.IsFormat("01234567-89AB-CDEF-0123-456789ABCDEF") {
		t.Error("Expected true, got false")
	}
	if !checker.IsFormat("F1234567-89AB-CDEF-0123-456789ABCDEF") {
		t.Error("Expected true, got false")
	}

	if checker.IsFormat("not-a-uuid") {
		t.Error("Expected false, got true")
	}

	if checker.IsFormat("g1234567-89ab-cdef-0123-456789abcdef") {
		t.Error("Expected false, got true")
	}
}

func TestURIReferenceFormatCheckerIsFormat(t *testing.T) {
	checker := URIReferenceFormatChecker{}

	if !checker.IsFormat("relative") {
		t.Error("Expected true, got false")
	}
	if !checker.IsFormat("https://dummyhost.com/dummy-path?dummy-qp-name=dummy-qp-value") {
		t.Error("Expected true, got false")
	}
}

const formatSchema = `{
	"type": "object",
	"properties": {
		"arr":  {"type": "array", "items": {"type": "string"}, "format": "ArrayChecker"},
		"bool": {"type": "boolean", "format": "BoolChecker"},
		"int":  {"format": "IntegerChecker"},
		"name": {"type": "string"},
		"str":  {"type": "string", "format": "StringChecker"}
	},
	"format": "ObjectChecker",
	"required": ["name"]
}`

type arrayChecker struct{}

func (c arrayChecker) IsFormat(input any) bool {
	arr, ok := input.([]any)
	if !ok {
		return true
	}
	for _, v := range arr {
		if v == "x" {
			return true
		}
	}
	return false
}

type boolChecker struct{}

func (c boolChecker) IsFormat(input any) bool {
	b, ok := input.(bool)
	if !ok {
		return true
	}
	return b
}

type integerChecker struct{}

func (c integerChecker) IsFormat(input any) bool {
	number, ok := input.(json.Number)
	if !ok {
		return true
	}
	f, _ := number.Float64()
	return int(f)%2 == 0
}

type objectChecker struct{}

func (c objectChecker) IsFormat(input any) bool {
	obj, ok := input.(map[string]any)
	if !ok {
		return true
	}
	return obj["name"] == "x"
}

type stringChecker struct{}

func (c stringChecker) IsFormat(input any) bool {
	str, ok := input.(string)
	if !ok {
		return true
	}
	return str == "o"
}

func TestCustomFormat(t *testing.T) {
	FormatCheckers.
		Add("ArrayChecker", arrayChecker{}).
		Add("BoolChecker", boolChecker{}).
		Add("IntegerChecker", integerChecker{}).
		Add("ObjectChecker", objectChecker{}).
		Add("StringChecker", stringChecker{})

	sl := NewStringLoader(formatSchema)
	validResult, err := Validate(sl, NewGoLoader(map[string]any{
		"arr":  []string{"x", "y", "z"},
		"bool": true,
		"int":  "2", // format not defined for string
		"name": "x",
		"str":  "o",
	}))
	if err != nil {
		t.Error(err)
	}

	if !validResult.Valid() {
		for _, desc := range validResult.Errors() {
			t.Error(desc)
		}
	}

	invalidResult, err := Validate(sl, NewGoLoader(map[string]any{
		"arr":  []string{"a", "b", "c"},
		"bool": false,
		"int":  1,
		"name": "z",
		"str":  "a",
	}))
	if err != nil {
		t.Error(err)
	}

	if len(invalidResult.Errors()) != 5 {
		t.Error("Expected 5 errors, got", len(invalidResult.Errors()))
	}

	FormatCheckers.
		Remove("ArrayChecker").
		Remove("BoolChecker").
		Remove("IntegerChecker").
		Remove("ObjectChecker").
		Remove("StringChecker")
}
