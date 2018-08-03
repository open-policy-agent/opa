// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build !linux,!darwin !cgo

package runtime

// Contains parts of the runtime package that do not use the plugin package.

// RegisterSharedObjectsFromDir is a no-op. Dynamically loading shared objects is limited to linux/darwin + cgo platforms.
func RegisterSharedObjectsFromDir(dir string) error {
	return nil
}
