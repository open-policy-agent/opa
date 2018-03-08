// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestRoundTrip(t *testing.T) {
	cases := []interface{}{
		nil,
		1,
		1.1,
		false,
		[]int{1},
		[]bool{true},
		[]string{"foo"},
		map[string]string{"foo": "bar"},
		struct {
			F string `json:"foo"`
			B int    `json:"bar"`
		}{"x", 32},
		map[string][]int{
			"ones": {1, 1, 1},
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("input %v", tc), func(t *testing.T) {
			err := util.RoundTrip(&tc)
			if err != nil {
				t.Errorf("expected error=nil, got %s", err.Error())
			}
			switch x := tc.(type) {
			// These are the output types we want, nothing else
			case nil, bool, json.Number, int64, float64, int, string, []interface{},
				[]string, map[string]interface{}, map[string]string:
			default:
				t.Error("unexpected type %T", x)
			}
		})
	}
}
