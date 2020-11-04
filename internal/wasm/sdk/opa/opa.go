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
	Result interface{}
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
		memoryMinPages: 2,
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

	defer o.pool.Release(instance)

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

	defer o.pool.Release(instance)

	return instance.Entrypoints(), nil
}

// EvalBool evaluates the boolean policy with the given input. The
// possible error values returned are as with Eval with addition of
// ErrUndefined indicating an undefined policy decision and
// ErrNonBoolean indicating a non-boolean policy decision.
// Deprecated: Use Eval instead.
func EvalBool(ctx context.Context, o *OPA, entrypoint EntrypointID, input *interface{}) (bool, error) {
	rs, err := o.Eval(ctx, EvalOpts{
		Entrypoint: entrypoint,
		Input:      input,
	})
	if err != nil {
		return false, err
	}

	r, ok := rs.Result.([]interface{})
	if !ok || len(r) == 0 {
		return false, ErrUndefined
	}

	m, ok := r[0].(map[string]interface{})
	if !ok || len(m) != 1 {
		return false, ErrNonBoolean
	}

	var b bool
	for _, v := range m {
		b, ok = v.(bool)
		break
	}

	if !ok {
		return false, ErrNonBoolean
	}

	return b, nil
}
