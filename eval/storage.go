// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import "fmt"
import "github.com/open-policy-agent/opa/opalog"

// StorageErrorCode represents the collection of error types that can be
// returned by Storage.
type StorageErrorCode int

const (
	// StorageInternalErr is used to represent an internal error has occurred. These
	// errors are unlikely to be recoverable.
	StorageInternalErr StorageErrorCode = iota

	// StorageNotFoundErr is used when a given reference does not locate a document
	// in Storage. In some cases, this may be recoverable.
	StorageNotFoundErr = iota

	// StorageNonGroundErr is used if the caller attempts to perform a Storage operation
	// on an unground reference.
	StorageNonGroundErr = iota
)

// StorageError is the error type returned by Storage functions.
type StorageError struct {

	// Code is used to identify the specific reason for the error.
	Code StorageErrorCode

	// Message can be displayed if the error is not recoverable.
	Message string
}

func (err *StorageError) Error() string {
	return fmt.Sprintf("storage error (code: %d): %v", err.Code, err.Message)
}

func notFoundError(f string, a ...interface{}) *StorageError {
	return &StorageError{
		Code:    StorageNotFoundErr,
		Message: fmt.Sprintf(f, a...),
	}
}

// Storage is the backend containing rules and data.
type Storage map[interface{}]interface{}

// NewStorageFromJSONObject returns Storage by converting from map[string]interface{}
func NewStorageFromJSONObject(data map[string]interface{}) Storage {
	store := Storage(map[interface{}]interface{}{})
	for k, v := range data {
		store[k] = v
	}
	return store
}

// Put inserts a value into storage.
func (store Storage) Put(path opalog.Ref, value interface{}) error {
	return nil
}

// Lookup returns the value in Storage referenced by path.
// If the lookup fails, an error is returned with a message indicating
// why the failure occurred.
func (store Storage) Lookup(path opalog.Ref) (interface{}, error) {

	if !path.IsGround() {
		return nil, &StorageError{Code: StorageNonGroundErr, Message: fmt.Sprintf("cannot lookup non-ground reference: %v", path)}
	}

	var node interface{} = store

	for i, v := range path {
		switch n := node.(type) {
		case Storage:
			// The first element in a reference is always a Var so we have
			// to handle this special case and use a type conversion from Var to string.
			r, ok := n[string(v.Value.(opalog.Var))]
			if !ok {
				return nil, notFoundError("cannot find path %v in storage, path references object missing key: %v", path, v)
			}
			node = r
		case map[string]interface{}:
			k, ok := v.Value.(opalog.String)
			if !ok {
				return nil, notFoundError("cannot find path %v in storage, path references object with non-string key: %v", path, v)
			}
			r, ok := n[string(k)]
			if !ok {
				return nil, notFoundError("cannot find path %v in storage, path references object missing key: %v", path, v)
			}
			node = r
		case []interface{}:
			k, ok := v.Value.(opalog.Number)
			if !ok {
				return nil, notFoundError("cannot find path %v in storage, path references array with non-numeric key: %v", path, v)
			}
			idx := int(k)
			if idx >= len(n) {
				return nil, notFoundError("cannot find path %v in storage, path references array with length: %v", path, len(n))
			} else if idx < 0 {
				return nil, notFoundError("cannot find path %v in storage, path references array using negative index: %v", path, idx)
			}
			node = n[idx]
		case *opalog.Rule:
			return nil, notFoundError("cannot find path %v in storage, path references rule at index: %v", path, i-1)
		default:
			return nil, notFoundError("cannot find path %v in storage, path references non-collection type at index: %v", path, i)
		}
	}

	return node, nil
}
