// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build !linux,!darwin !cgo

package runtime

// Contains parts of the runtime package that do not use the plugin package.

// RegisterBuiltinsFromDir is a no-op. Plugin loading is limited to linux/darwin + cgo platforms for the time being.
func RegisterBuiltinsFromDir(dir string) error {
	return nil
}

// RegisterPluginsFromDir is a no-op. Plugin loading is limited to linux/darwin + cgo platforms for the time being.
func RegisterPluginsFromDir(dir string) error {
	return nil
}
