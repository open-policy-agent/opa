// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package file

import (
	"fmt"
	"time"

	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
)

// WithFile configures the file to load the bundle from.
func (l *Loader) WithFile(filename string) *Loader {
	l.filename = filename
	return l
}

// WithInterval configures the delay between bundle file reloading.
func (l *Loader) WithInterval(interval time.Duration) *Loader {
	l.interval = interval
	return l
}

// WithErrorLogger configures an error logger invoked with all the errors.
func (l *Loader) WithErrorLogger(logger func(error)) *Loader {
	if logger == nil {
		l.configErr = fmt.Errorf("logger: %w", opa.ErrInvalidConfig)
		return l
	}

	l.logError = logger
	return l
}
