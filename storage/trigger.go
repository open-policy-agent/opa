// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

// TriggerCallback defines the interface that callers can implement to handle
// changes in the stores.
type TriggerCallback func(txn Transaction, op PatchOp, path Path, value interface{}) error

// TriggerConfig contains the trigger registration configuration.
type TriggerConfig struct {

	// Before is called before the change is applied to the store.
	Before TriggerCallback

	// After is called after the change is applied to the store.
	After TriggerCallback

	// TODO(tsandall): include callbacks for aborted changes
}

// Trigger defines the interface that stores implement to register for change
// notifications when data in the store changes.
type Trigger interface {
	Register(id string, config TriggerConfig) error

	// Unregister instructs the trigger to remove the registration.
	Unregister(id string)
}

// TriggersNotSupported provides default implementations of the Trigger
// interface which may be used if the backend does not support triggers.
type TriggersNotSupported struct{}

// Register always returns an error indicating triggers are not supported.
func (TriggersNotSupported) Register(string, TriggerConfig) error {
	return triggersNotSupportedError()
}

// Unregister is a no-op.
func (TriggersNotSupported) Unregister(string) {

}
