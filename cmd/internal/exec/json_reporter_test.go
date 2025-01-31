package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/open-policy-agent/opa/v1/sdk"
)

func TestJsonReporter_Close(t *testing.T) {
	wr := bytes.NewBuffer([]byte{})
	wrp := &wr
	testString := "test"
	testData := []result{
		{Path: testString},
	}
	jr := jsonReporter{w: *wrp, buf: testData}
	if err := jr.Close(); err != nil {
		t.Fatalf("unexpected error running jsonReporter.Close: %q", err.Error())
	}
	results := struct {
		Result []result
	}{}
	if err := json.Unmarshal(wr.Bytes(), &results); err != nil {
		t.Fatalf("unexpected error deserializing results: %q", err.Error())
	}
	if results.Result[0].Path != testString {
		t.Fatalf("expected result Path to be %q, got %q", testString, results.Result[0].Path)
	}
}

func TestJsonReporter_StoreDecision(t *testing.T) {
	testString := "test"
	ctx := context.TODO()
	tcs := []struct {
		Name                 string
		Path                 string
		DecisionFunc         func(ctx context.Context, options sdk.DecisionOptions) (*sdk.DecisionResult, error)
		Params               Params
		ExpectedErrorCount   int
		ExpectedFailureCount int
	}{
		{
			Name: "should return nil with increased error count if error is raised from decision",
			Path: testString,
			DecisionFunc: func(_ context.Context, _ sdk.DecisionOptions) (*sdk.DecisionResult, error) {
				return nil, errors.New("test")
			},
			Params:               Params{FailNonEmpty: true},
			ExpectedErrorCount:   1,
			ExpectedFailureCount: 0,
		},
		{
			Name: "should increase failure count if decision result is nil and params.Fail is true",
			Path: testString,
			DecisionFunc: func(_ context.Context, _ sdk.DecisionOptions) (*sdk.DecisionResult, error) {
				return &sdk.DecisionResult{Result: nil}, nil
			},
			Params:               Params{Fail: true},
			ExpectedErrorCount:   0,
			ExpectedFailureCount: 1,
		},
		{
			Name: "should increase failure count by 2 if decision result is not nil and params.FailDefined and params.FailNonEmpty are true",
			Path: testString,
			DecisionFunc: func(_ context.Context, _ sdk.DecisionOptions) (*sdk.DecisionResult, error) {
				return &sdk.DecisionResult{Result: []string{testString}}, nil
			},
			Params:               Params{FailDefined: true, FailNonEmpty: true},
			ExpectedErrorCount:   0,
			ExpectedFailureCount: 2,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			wr := bytes.NewBuffer([]byte{})
			j := jsonReporter{
				w:            wr,
				buf:          []result{},
				decisionFunc: tc.DecisionFunc,
				params:       &tc.Params,
				ctx:          &ctx,
			}
			j.StoreDecision(nil, testString)
			if j.errorCount != tc.ExpectedErrorCount {
				t.Fatalf("expected error count to be %d, got %d", tc.ExpectedErrorCount, j.errorCount)
			}
			if j.failCount != tc.ExpectedFailureCount {
				t.Fatalf("expected failure count to be %d, got %d", tc.ExpectedFailureCount, j.failCount)
			}
		})
	}
}

func TestJsonReporter_ReportFailure(t *testing.T) {
	tcs := []struct {
		Name   string
		Params Params
		Errs   int
		Fails  int
		IsErr  bool
	}{
		{
			Name:   "errors with Fail flagged",
			Params: Params{Fail: true},
			Errs:   5,
			IsErr:  true,
		},
		{
			Name:   "failures with FailDefined flagged",
			Params: Params{FailDefined: true},
			Fails:  3,
			IsErr:  true,
		},
		{
			Name:   "failures and errors with FailNonEmpty flagged",
			Params: Params{FailNonEmpty: true},
			Fails:  1,
			Errs:   1,
			IsErr:  true,
		},
		{
			Name:   "no failures nor errors",
			Params: Params{Fail: true},
			Fails:  0,
			Errs:   0,
			IsErr:  false,
		},
		{
			Name:   "failures and errors without param flags",
			Params: Params{},
			Fails:  2,
			Errs:   2,
			IsErr:  false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			wr := bytes.NewBuffer([]byte{})
			ctx := context.Background()
			j := jsonReporter{
				w:   wr,
				buf: []result{},
				decisionFunc: func(_ context.Context, _ sdk.DecisionOptions) (*sdk.DecisionResult, error) {
					return &sdk.DecisionResult{}, nil
				},
				params: &tc.Params,
				ctx:    &ctx,
			}
			j.errorCount = tc.Errs
			j.failCount = tc.Fails
			if err := j.ReportFailure(); tc.IsErr && err == nil {
				t.Fatalf("expected error, found none")
			} else if !tc.IsErr && err != nil {
				t.Fatalf("unexpected error: %q", err.Error())
			}
		})
	}
}
