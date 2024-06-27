package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/open-policy-agent/opa/sdk"
)

type result struct {
	Path   string       `json:"path"`
	Error  error        `json:"error,omitempty"`
	Result *interface{} `json:"result,omitempty"`
}

type jsonReporter struct {
	w            io.Writer
	buf          []result
	ctx          *context.Context
	opa          *sdk.OPA
	params       *Params
	errorCount   int
	failCount    int
	decisionFunc func(ctx context.Context, options sdk.DecisionOptions) (*sdk.DecisionResult, error)
}

func (jr *jsonReporter) Report(r result) {
	jr.buf = append(jr.buf, r)
}

func (jr *jsonReporter) Close() error {
	enc := json.NewEncoder(jr.w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Result []result `json:"result"`
	}{
		Result: jr.buf,
	})
}

func (jr *jsonReporter) StoreDecision(input *interface{}, itemPath string) {
	rs, err := jr.decisionFunc(*jr.ctx, sdk.DecisionOptions{
		Path:  jr.params.Decision,
		Now:   time.Now(),
		Input: input,
	})
	if err != nil {
		jr.Report(result{Path: itemPath, Error: err})
		if (jr.params.FailDefined && !sdk.IsUndefinedErr(err)) || (jr.params.Fail && sdk.IsUndefinedErr(err)) || (jr.params.FailNonEmpty && !sdk.IsUndefinedErr(err)) {
			jr.errorCount++
		}
		return
	}

	jr.Report(result{Path: itemPath, Result: &rs.Result})

	if (jr.params.FailDefined && rs.Result != nil) || (jr.params.Fail && rs.Result == nil) {
		jr.failCount++
	}

	if jr.params.FailNonEmpty && rs.Result != nil {
		// Check if rs.Result is an array and has one or more members
		resultArray, isArray := rs.Result.([]interface{})
		if (!isArray) || (isArray && (len(resultArray) > 0)) {
			jr.failCount++
		}
	}
}

func (jr *jsonReporter) ReportFailure() error {
	if (jr.params.Fail || jr.params.FailDefined || jr.params.FailNonEmpty) && (jr.failCount > 0 || jr.errorCount > 0) {
		if jr.params.Fail {
			return fmt.Errorf("there were %d failures and %d errors counted in the results list, and --fail is set", jr.failCount, jr.errorCount)
		}
		if jr.params.FailDefined {
			return fmt.Errorf("there were %d failures and %d errors counted in the results list, and --fail-defined is set", jr.failCount, jr.errorCount)
		}
		return fmt.Errorf("there were %d failures and %d errors counted in the results list, and --fail-non-empty is set", jr.failCount, jr.errorCount)
	}

	return nil
}
