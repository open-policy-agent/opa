// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

const wasmPageSize = 65535

// WithPolicyFile configures a policy file to load.
func (o *OPA) WithPolicyFile(fileName string) *OPA {
	policy, err := ioutil.ReadFile(fileName)
	if err != nil {
		o.configErr = fmt.Errorf("%v: %w", err.Error(), ErrInvalidConfig)
		return o
	}

	o.policy = policy
	return o
}

// WithPolicyBytes configures the compiled policy to load.
func (o *OPA) WithPolicyBytes(policy []byte) *OPA {
	o.policy = policy
	return o
}

// WithDataFile configures the JSON data file to load.
func (o *OPA) WithDataFile(fileName string) *OPA {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		o.configErr = fmt.Errorf("%v: %w", err.Error(), ErrInvalidConfig)
		return o
	}

	o.data = data
	return o
}

// WithDataBytes configures the JSON data to load.
func (o *OPA) WithDataBytes(data []byte) *OPA {
	o.data = data
	return o
}

// WithDataJSON configures the JSON data to load.
func (o *OPA) WithDataJSON(data interface{}) *OPA {
	v, err := json.Marshal(data)
	if err != nil {
		o.configErr = fmt.Errorf("%v: %w", err.Error(), ErrInvalidConfig)
		return o
	}

	o.data = v
	return o
}

// WithMemoryLimits configures the memory limits (in bytes) for a single policy
// evaluation.
func (o *OPA) WithMemoryLimits(min, max uint32) *OPA {
	if min < 2*65535 {
		o.configErr = fmt.Errorf("too low minimum memory limit: %w", ErrInvalidConfig)
		return o
	}

	if max != 0 && min > max {
		o.configErr = fmt.Errorf("too low maximum memory limit: %w", ErrInvalidConfig)
		return o
	}

	o.memoryMinPages, o.memoryMaxPages = pages(min), pages(max)
	return o
}

// WithPoolSize configures the maximum number of simultaneous policy
// evaluations, i.e., the maximum number of underlying WASM instances
// active at any time. The default is the number of logical CPUs
// usable for the process as per runtime.NumCPU().
func (o *OPA) WithPoolSize(size uint32) *OPA {
	if size == 0 {
		o.configErr = fmt.Errorf("pool size: %w", ErrInvalidConfig)
		return o
	}

	o.poolSize = size
	return o
}

// WithErrorLogger configures an error logger invoked with all the errors.
func (o *OPA) WithErrorLogger(logger func(error)) *OPA {
	o.logError = logger
	return o
}

// pages converts a byte size to pages, rounding up as necessary.
func pages(n uint32) uint32 {
	pages := n / wasmPageSize
	if pages*wasmPageSize == n {
		return pages
	}

	return pages + 1
}
