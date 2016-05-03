// Copyright 2013 Apcera Inc. All rights reserved.

// +build !cgo windows

package term

// Used when we have no other source for getting platform-specific information
// about the terminal sizes available.

import (
	"errors"
	"os"
)

// ErrNoPlatformSizes indicates we don't have platform-specific sizing information.
var ErrNoPlatformSizes = errors.New("term: no platform-specific sizes available")

// GetTerminalWindowSize returns ErrNoPlatformSizes as an error, always, for
// this default implementation.
func GetTerminalWindowSize(file *os.File) (*Size, error) {
	return nil, ErrNoPlatformSizes
}
