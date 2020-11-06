// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opa

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"

	"github.com/open-policy-agent/opa/metrics"
)

// OPA executes WebAssembly compiled Rego policies.
type OPA struct {
	configErr      error // Delayed configuration error, if any.
	memoryMinPages uint32
	memoryMaxPages uint32 // 0 means no limit.
	poolSize       uint32
	pool           *pool
	mutex          sync.Mutex // To serialize access to SetPolicy, SetData and Close.
	policy         []byte     // Current policy.
	data           []byte     // Current data.
	logError       func(error)
}

// Result holds the evaluation result.
type Result struct {
	Result []byte
}

// EntrypointID is used by Eval() to determine which compiled entrypoint should
// be evaluated. Retrieve entrypoint to ID mapping for each instance of the
// compiled policy.
type EntrypointID int32

// New constructs a new OPA SDK instance, ready to be configured with
// With functions. If no policy is provided as a part of
// configuration, policy (and data) needs to be set before invoking
// Eval. Once constructed and configured, the instance needs to be
// initialized before invoking the Eval.
func New() *OPA {
	opa := &OPA{
		memoryMinPages: 16,
		memoryMaxPages: 0,
		poolSize:       uint32(runtime.GOMAXPROCS(0)),
		logError:       func(error) {},
	}

	return opa
}

// Init initializes the SDK instance after the construction and
// configuration. If the configuration is invalid, it returns
// ErrInvalidConfig.
func (o *OPA) Init() (*OPA, error) {
	if o.configErr != nil {
		return nil, o.configErr
	}

	o.pool = newPool(o.poolSize, o.memoryMinPages, o.memoryMaxPages)

	if len(o.policy) != 0 {
		if err := o.pool.SetPolicyData(o.policy, o.data); err != nil {
			return nil, err
		}
	}

	return o, nil
}

// SetData updates the data for the subsequent Eval calls.  Returns
// either ErrNotReady, ErrInvalidPolicyOrData, or ErrInternal if an
// error occurs.
func (o *OPA) SetData(v interface{}) error {
	if o.pool == nil {
		return ErrNotReady
	}

	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("%v: %w", err, ErrInvalidPolicyOrData)
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.setPolicyData(o.policy, raw)
}

// SetDataPath will update the current data on the VMs by setting the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (o *OPA) SetDataPath(path []string, value interface{}) error {
	return o.pool.SetDataPath(path, value)
}

// RemoveDataPath will update the current data on the VMs by removing the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (o *OPA) RemoveDataPath(path []string) error {
	return o.pool.RemoveDataPath(path)
}

// SetPolicy updates the policy for the subsequent Eval calls.
// Returns either ErrNotReady, ErrInvalidPolicy or ErrInternal if an
// error occurs.
func (o *OPA) SetPolicy(p []byte) error {
	if o.pool == nil {
		return ErrNotReady
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.setPolicyData(p, o.data)
}

// SetPolicyData updates both the policy and data for the subsequent
// Eval calls.  Returns either ErrNotReady, ErrInvalidPolicyOrData, or
// ErrInternal if an error occurs.
func (o *OPA) SetPolicyData(policy []byte, data *interface{}) error {
	if o.pool == nil {
		return ErrNotReady
	}

	var raw []byte
	if data != nil {
		var err error
		raw, err = json.Marshal(*data)
		if err != nil {
			return fmt.Errorf("%v: %w", err, ErrInvalidPolicyOrData)
		}
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.setPolicyData(policy, raw)
}

func (o *OPA) setPolicyData(policy []byte, data []byte) error {
	if err := o.pool.SetPolicyData(policy, data); err != nil {
		return err
	}

	o.policy = policy
	o.data = data
	return nil
}

// EvalOpts define options for performing an evaluation
type EvalOpts struct {
	Entrypoint EntrypointID
	Input      *interface{}
	Metrics    metrics.Metrics
}

// Eval evaluates the policy with the given input, returning the
// evaluation results. If no policy was configured at construction
// time nor set after, the function returns ErrNotReady.  It returns
// ErrInternal if any other error occurs.
func (o *OPA) Eval(ctx context.Context, opts EvalOpts) (*Result, error) {
	if o.pool == nil {
		return nil, ErrNotReady
	}

	m := opts.Metrics
	if m == nil {
		m = metrics.New()
	}

	instance, err := o.pool.Acquire(ctx, m)
	if err != nil {
		return nil, err
	}

	defer o.pool.Release(instance, m)

	result, err := instance.Eval(ctx, opts.Entrypoint, opts.Input, m)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", err, ErrInternal)
	}

	return &Result{result}, nil
}

// Close waits until all the pending evaluations complete and then
// releases all the resources allocated. Eval will return ErrClosed
// afterwards.
func (o *OPA) Close() {
	if o.pool == nil {
		return
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.pool.Close()
}

// Entrypoints returns a mapping of entrypoint name to ID for use by Eval() and EvalBool().
func (o *OPA) Entrypoints(ctx context.Context) (map[string]EntrypointID, error) {
	instance, err := o.pool.Acquire(ctx, metrics.New())
	if err != nil {
		return nil, err
	}

	defer o.pool.Release(instance, metrics.New())

	return instance.Entrypoints(), nil
}
