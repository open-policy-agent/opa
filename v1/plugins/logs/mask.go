// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/internal/deepcopy"
)

type maskOP string

const (
	maskOPRemove maskOP = "remove"
	maskOPUpsert maskOP = "upsert"

	partInput    = "input"
	partResult   = "result"
	partNDBCache = "nd_builtin_cache"
)

var errMaskInvalidObject = errors.New("mask upsert invalid object")

type maskRule struct {
	OP                maskOP      `json:"op"`
	Path              string      `json:"path"`
	Value             interface{} `json:"value"`
	escapedParts      []string
	modifyFullObj     bool
	failUndefinedPath bool
}

type maskRuleSet struct {
	OnRuleError  func(*maskRule, error)
	Rules        []*maskRule
	resultCopied bool
}

func (r maskRule) String() string {
	return "/" + strings.Join(r.escapedParts, "/")
}

type maskRuleOption func(*maskRule) error

func newMaskRule(path string, opts ...maskRuleOption) (*maskRule, error) {
	const (
		defaultOP                = maskOPRemove
		defaultFailUndefinedPath = false
	)

	if len(path) == 0 {
		return nil, errors.New("mask must be non-empty")
	} else if !strings.HasPrefix(path, "/") {
		return nil, errors.New("mask must be slash-prefixed")
	}

	parts := strings.Split(path[1:], "/")

	switch parts[0] {
	case partInput, partResult, partNDBCache: // OK
	default:
		return nil, fmt.Errorf("mask prefix not allowed: %v", parts[0])
	}

	escapedParts := make([]string, len(parts))
	for i := range parts {
		_, err := url.PathUnescape(parts[i])
		if err != nil {
			return nil, err
		}

		escapedParts[i] = url.PathEscape(parts[i])
	}

	var modifyFullObj bool
	if len(escapedParts) == 1 {
		modifyFullObj = true
	}

	r := &maskRule{
		OP:                defaultOP,
		Path:              path,
		escapedParts:      escapedParts,
		failUndefinedPath: defaultFailUndefinedPath,
		modifyFullObj:     modifyFullObj,
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func withOP(op maskOP) maskRuleOption {
	return func(r *maskRule) error {
		switch op {
		case maskOPRemove, maskOPUpsert:
			r.OP = op
			return nil
		}
		return fmt.Errorf("mask op is not supported: %s", op)
	}
}

func withValue(val interface{}) maskRuleOption {
	return func(r *maskRule) error {
		r.Value = val
		return nil
	}
}

func withFailUndefinedPath() maskRuleOption {
	return func(r *maskRule) error {
		r.failUndefinedPath = true
		return nil
	}
}

func (r maskRule) Mask(event *EventV1) error {
	var maskObj *interface{}     // pointer to event Input|Result|NDBCache object
	var maskObjPtr **interface{} // pointer to the event Input|Result|NDBCache pointer itself

	switch p := r.escapedParts[0]; p {
	case partInput:
		if event.Input == nil {
			if r.failUndefinedPath {
				return errMaskInvalidObject
			}
			return nil
		}
		maskObj = event.Input
		maskObjPtr = &event.Input
	case partResult:
		if event.Result == nil {
			if r.failUndefinedPath {
				return errMaskInvalidObject
			}
			return nil
		}
		maskObj = event.Result
		maskObjPtr = &event.Result
	case partNDBCache:
		if event.NDBuiltinCache == nil {
			if r.failUndefinedPath {
				return errMaskInvalidObject
			}
			return nil
		}
		maskObj = event.NDBuiltinCache
		maskObjPtr = &event.NDBuiltinCache
	default:
		return fmt.Errorf("illegal path value: %s", p)
	}

	switch r.OP {
	case maskOPRemove:
		if r.modifyFullObj {
			*maskObjPtr = nil
		} else {
			err := r.removeValue(r.escapedParts[1:], *maskObj)
			if err != nil {
				if err == errMaskInvalidObject && r.failUndefinedPath {
					return err
				}
				return nil
			}
		}

		event.Erased = append(event.Erased, r.String())
	case maskOPUpsert:
		if r.modifyFullObj {
			*maskObjPtr = &r.Value
		} else {
			inputObj, ok := (*maskObj).(map[string]interface{})
			if !ok {
				return nil
			}

			if err := r.mkdirp(inputObj, r.escapedParts[1:len(r.escapedParts)], r.Value); err != nil {
				if r.failUndefinedPath {
					return err
				}

				return nil
			}
		}

		event.Masked = append(event.Masked, r.String())
	default:
		return fmt.Errorf("illegal mask op value: %s", r.OP)
	}

	return nil
}

func (r maskRule) removeValue(p []string, node interface{}) error {
	if len(p) == 0 {
		return nil
	}

	// the key or index to be removed
	targetKey := p[len(p)-1]

	// nodeParent stores the parent of the node to be modified during the
	// removal, this is only needed when the node is a slice
	var nodeParent interface{}
	// nodeKey stores the key of the node to be modified relative to the parent
	var nodeKey string

	// Walk to the parent of the target to be removed, the nodeParent is cached
	// support removing of slice values
	for i := range len(p) - 1 {
		switch v := node.(type) {
		case map[string]interface{}:
			child, ok := v[p[i]]
			if !ok {
				return errMaskInvalidObject
			}
			nodeParent = v
			nodeKey = p[i]
			node = child

		case []interface{}:
			index, err := strconv.Atoi(p[i])
			if err != nil || index < 0 || index >= len(v) {
				return errMaskInvalidObject
			}
			nodeParent = v
			nodeKey = p[i]
			node = v[index]

		default:
			return errMaskInvalidObject
		}
	}

	switch v := node.(type) {
	case map[string]interface{}:
		if _, ok := v[targetKey]; !ok {
			return errMaskInvalidObject
		}

		delete(v, targetKey)

	case []interface{}:
		// first, check the targetKey is a valid index
		targetIndex, err := strconv.Atoi(targetKey)
		if err != nil || targetIndex < 0 || targetIndex >= len(v) {
			return errMaskInvalidObject
		}

		switch nodeParent := nodeParent.(type) {
		case []interface{}:
			// update the target's grandparent slice with a new slice
			index, err := strconv.Atoi(nodeKey)
			if err != nil {
				return errMaskInvalidObject
			}

			nodeParent[index] = append(v[:targetIndex], v[targetIndex+1:]...)

		case map[string]interface{}:
			nodeParent[nodeKey] = append(v[:targetIndex], v[targetIndex+1:]...)

		default:
			return errMaskInvalidObject
		}

	default:
		return errMaskInvalidObject
	}

	return nil
}

func (r maskRule) mkdirp(node interface{}, path []string, value interface{}) error {
	if len(path) == 0 {
		return nil
	}

	for i := range len(path) - 1 {
		switch v := node.(type) {
		case map[string]interface{}:
			child, ok := v[path[i]]
			if !ok {
				child = map[string]interface{}{}
				v[path[i]] = child
			}

			node = child

		case []interface{}:
			idx, err := strconv.Atoi(path[i])
			if err != nil || idx < 0 {
				return errMaskInvalidObject
			}

			for len(v) <= idx {
				v = append(v, nil)
			}

			node = v[idx]

		default:
			return errMaskInvalidObject
		}
	}

	switch v := node.(type) {
	case map[string]interface{}:
		v[path[len(path)-1]] = value

	case []interface{}:
		idx, err := strconv.Atoi(path[len(path)-1])
		if err != nil || idx < 0 || idx >= len(v) {
			return errMaskInvalidObject
		}
		v[idx] = value

	default:
		return errMaskInvalidObject
	}

	return nil
}

func newMaskRuleSet(rv interface{}, onRuleError func(*maskRule, error)) (*maskRuleSet, error) {
	mRuleSet := &maskRuleSet{
		OnRuleError: onRuleError,
	}
	rawRules, ok := rv.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected rule format %v (%[1]T)", rv)
	}

	for _, iface := range rawRules {
		switch v := iface.(type) {

		case string:
			// preserve default behavior of remove when
			// structured mask format is not provided
			rule, err := newMaskRule(v)
			if err != nil {
				return nil, err
			}

			mRuleSet.Rules = append(mRuleSet.Rules, rule)

		case map[string]interface{}:
			rule := &maskRule{}
			op, set := getString(v, "op")
			if set && op == "" {
				return nil, fmt.Errorf("invalid \"op\" value: %v %[1]T", v["op"])
			}
			rule.OP = maskOP(op)

			path, set := getString(v, "path")
			if set && path == "" {
				return nil, fmt.Errorf("invalid \"path\" value: %v %[1]T", v["path"])
			}
			rule.Path = path

			rule.Value = v["value"]

			// use unmarshalled values to create new Mask Rule
			rule, err := newMaskRule(rule.Path, withOP(rule.OP), withValue(rule.Value))
			// TODO add withFailUndefinedPath() option based on
			//   A) new syntax in user defined mask rule
			//   B) passed in/global configuration option
			//   rule precedence A>B
			if err != nil {
				return nil, err
			}

			mRuleSet.Rules = append(mRuleSet.Rules, rule)

		default:
			return nil, fmt.Errorf("invalid mask rule format encountered: %T", v)
		}
	}

	return mRuleSet, nil
}

func (rs maskRuleSet) Mask(event *EventV1) {
	for _, mRule := range rs.Rules {
		// result must be deep copied if there are any mask rules
		// targeting it, to avoid modifying the result sent
		// to the consumer
		if mRule.escapedParts[0] == partResult && event.Result != nil && !rs.resultCopied {
			resultCopy := deepcopy.DeepCopy(*event.Result)
			event.Result = &resultCopy
			rs.resultCopied = true
		}
		err := mRule.Mask(event)
		if err != nil {
			rs.OnRuleError(mRule, err)
		}
	}
}

// bool return means the field was set, if the string is still "", the
// value was invalid
func getString(x map[string]any, key string) (string, bool) {
	y, ok := x[key]
	if !ok {
		return "", false
	}
	s, ok := y.(string)
	if !ok {
		return "", true
	}
	return s, true
}
