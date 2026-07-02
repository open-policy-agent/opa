// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tlsutil

import "testing"

func TestBuildTLSConfigOff(t *testing.T) {
	tlsConfig, err := BuildTLSConfig("off", false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig != nil {
		t.Fatal("expected nil tls config for 'off' encryption")
	}
}

func TestBuildTLSConfigMTLSNoCert(t *testing.T) {
	_, err := BuildTLSConfig("mtls", false, nil, nil)
	if err == nil {
		t.Fatal("expected error for mtls without cert")
	}
}
