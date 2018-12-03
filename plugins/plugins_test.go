// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package plugins

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/storage/inmem"
)

func TestMangerUpdate(t *testing.T) {
	managerConfig := []byte(fmt.Sprintf(`{
			"services": [
				{
					"name": "example",
					"url": "example.com",
					"credentials": {
						"bearer": {
							"scheme": "Bearer",
							"token": "secret"
						}
					}
				}
			],
			"labels": {
				"app": "myapp",
				"environment": "production"
			}}`))

	store := inmem.New()

	manager, err := New(managerConfig, "test-instance-id", store)
	if err != nil {
		t.Fatal(err)
	}

	updatedConfig := []byte(fmt.Sprintf(`{
			"services": [
				{
					"name": "example",
					"url": "example.io",
					"credentials": {
						"bearer": {
							"scheme": "Bearer",
							"token": "secret"
						}
					}
				},
				{
					"name": "blah",
					"url": "blah.com"
				}
			],
			"labels": {
				"app": "myapp",
				"region": "west",
				"environment": "dev"
			}}`))

	err = manager.Update(updatedConfig)
	if err != nil {
		t.Fatal(err)
	}

	expectedLabels := map[string]string{}
	expectedLabels["id"] = "test-instance-id"
	expectedLabels["app"] = "myapp"
	expectedLabels["region"] = "west"
	expectedLabels["environment"] = "dev"

	expectedServices := []string{"example", "blah"}

	if !reflect.DeepEqual(expectedLabels, manager.Labels) {
		t.Fatalf("Expected labels %v, but got %v", expectedLabels, manager.Labels)
	}

	if len(expectedServices) != len(manager.Services()) {
		t.Fatalf("Expected services %v, but got %v", expectedServices, manager.Services())
	}

}
