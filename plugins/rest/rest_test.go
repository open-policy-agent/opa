// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"testing"
)

func TestNew(t *testing.T) {

	tests := []struct {
		input   string
		wantErr bool
	}{
		{
			input: `{
				"name": "foo",
				"url": "bad scheme://authority",
			}`,
			wantErr: true,
		},
		{
			input: `{
				"name": "foo",
				"url", "http://localhost/some/path",
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"bearer": {
						"token": "secret",
					}
				}
			}`,
		},
		{
			input: `{
				"name": "foo",
				"url": "http://localhost",
				"credentials": {
					"bearer": {
						"scheme": "Acmecorp-Token",
						"token": "secret"
					}
				}
			}`,
		},
	}

	var results []Client

	for _, tc := range tests {
		client, err := New([]byte(tc.input))
		if err != nil && !tc.wantErr {
			t.Fatalf("Unexpected error: %v", err)
		}
		results = append(results, client)
	}

	if results[2].config.Credentials.Bearer.Scheme != "Bearer" {
		t.Fatalf("Expected token scheme to be set to Bearer but got: %v", results[2].config.Credentials.Bearer.Scheme)
	}

	if results[3].config.Credentials.Bearer.Scheme != "Acmecorp-Token" {
		t.Fatalf("Expected custom token but got: %v", results[3].config.Credentials.Bearer.Scheme)
	}

}
