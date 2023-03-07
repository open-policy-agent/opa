// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package status

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/open-policy-agent/opa/metrics"
)

const (
	errCode = "decision_log_error"
)

// Status represents the status of processing a decision log.
type Status struct {
	Code     string          `json:"code,omitempty"`
	Message  string          `json:"message,omitempty"`
	HTTPCode json.Number     `json:"http_code,omitempty"`
	Metrics  metrics.Metrics `json:"metrics,omitempty"`
}

// SetError updates the status object to reflect a failure to upload or
// process a log. If err is nil, the error status is cleared.
func (s *Status) SetError(err error) {
	var httpError HTTPError

	switch {
	case err == nil:
		s.Code = ""
		s.HTTPCode = ""
		s.Message = ""

	case errors.As(err, &httpError):
		s.Code = errCode
		s.HTTPCode = json.Number(strconv.Itoa(httpError.StatusCode))
		s.Message = err.Error()

	default:
		s.Code = errCode
		s.HTTPCode = ""
		s.Message = err.Error()
	}
}

type HTTPError struct {
	StatusCode int
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("log upload failed, server replied with HTTP %v %v", e.StatusCode, http.StatusText(e.StatusCode))
}
