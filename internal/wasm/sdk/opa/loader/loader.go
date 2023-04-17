// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package loader

import (
	"context"
)

// Loader is the interface all bundle loaders implement.
type Loader interface {
	// Load loads a bundle. This can be invoked without starting the polling.
	Load(ctx context.Context) error

	// Start starts the bundle polling.
	Start(ctx context.Context) error

	// Close stops the polling.
	Close()
}
