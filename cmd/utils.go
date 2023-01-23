// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
)

type ExitError struct {
	Exit int
}

func newExitError(exit int) error {
	return &ExitError{Exit: exit}
}

func (c *ExitError) Error() string {
	return fmt.Sprintf("exit %d", c.Exit)
}
