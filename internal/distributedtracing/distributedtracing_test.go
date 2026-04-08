// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package distributedtracing

import "testing"

func TestInitNoTypeReturnsAllNil(t *testing.T) {
	raw := []byte(`{"distributed_tracing": {}}`)
	exp, tp, res, err := Init(t.Context(), raw, "test")
	if err != nil {
		t.Fatal(err)
	}
	if exp != nil || tp != nil || res != nil {
		t.Fatal("expected all nil when type is not set")
	}
}
