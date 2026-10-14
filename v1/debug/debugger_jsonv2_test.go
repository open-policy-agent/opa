//go:build go1.27

package debug_test

import (
	"testing"

	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/open-policy-agent/opa/v1/debug"
)

func TestUnmarshalLaunchEvalProperties(t *testing.T) {
	var props debug.LaunchEvalProperties

	err := json.Unmarshal(jsontext.Value(`{
		"query": "data.example.allow = true",
		"input": {"x": 1},
		"inputPath": "/foo/bar",
		"bundlePaths": ["/foo/bundle"],
		"dataPaths": ["/foo/data"],
		"stopOnResult": true,
		"stopOnEntry": true,
		"stopOnFail": true,
		"enablePrint": true,
		"skipOps": null,
		"strictBuiltinErrors": true,
		"ruleIndexing": true,
		"stackTraceMode": "query"
	}`), &props)
	if err != nil {
		t.Fatalf("failed to unmarshal LaunchEvalProperties: %v", err)
	}

	if props.Query != "data.example.allow = true" {
		t.Errorf("expected Query to be 'data.example.allow = true', got %q", props.Query)
	}
	input, ok := props.Input.(map[string]any)
	if !ok {
		t.Errorf("expected Input type to be map[string]any, got %T", props.Input)
	}
	if input["x"].(float64) != 1 {
		t.Errorf("expected Input[\"x\"] to be 1, got %v", input["x"])
	}
	if props.InputPath != "/foo/bar" {
		t.Errorf("expected InputPath to be '/foo/bar', got %q", props.InputPath)
	}
	if len(props.BundlePaths) != 1 || props.BundlePaths[0] != "/foo/bundle" {
		t.Errorf("expected BundlePaths to be ['/foo/bundle'], got %v", props.BundlePaths)
	}
	if len(props.DataPaths) != 1 || props.DataPaths[0] != "/foo/data" {
		t.Errorf("expected DataPaths to be ['/foo/data'], got %v", props.DataPaths)
	}
	if !props.StopOnResult {
		t.Errorf("expected StopOnResult to be true, got %v", props.StopOnResult)
	}
	if !props.StopOnEntry {
		t.Errorf("expected StopOnEntry to be true, got %v", props.StopOnEntry)
	}
	if !props.StopOnFail {
		t.Errorf("expected StopOnFail to be true, got %v", props.StopOnFail)
	}
	if !props.EnablePrint {
		t.Errorf("expected EnablePrint to be true, got %v", props.EnablePrint)
	}
	if props.SkipOps != nil {
		t.Errorf("expected SkipOps to be nil, got %v", props.SkipOps)
	}
	if !props.StrictBuiltinErrors {
		t.Errorf("expected StrictBuiltinErrors to be true, got %v", props.StrictBuiltinErrors)
	}
	if !props.RuleIndexing {
		t.Errorf("expected RuleIndexing to be true, got %v", props.RuleIndexing)
	}
	if props.StackTraceMode != debug.StackTraceModeQuery {
		t.Errorf("expected StackTraceMode to be StackTraceModeQuery, got %v", props.StackTraceMode)
	}

}
