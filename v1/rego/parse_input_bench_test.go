// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/metrics"
)

// BenchmarkParseRawInput exercises parseRawInput on inputs already in
// JSON-native form.
//
// With util.RoundTrip:
// leaves=10-16        7811 ns/op    9467 B/op    174 allocs/op
// leaves=100-16      60938 ns/op   75236 B/op   1360 allocs/op
// leaves=1000-16    639872 ns/op  829139 B/op  13578 allocs/op
// leaves=10000-16  6139634 ns/op 8607215 B/op 139664 allocs/op
//
// With util.RoundTripFast:
// leaves=10-16        2915 ns/op    7128 B/op     98 allocs/op
// leaves=100-16      21939 ns/op   55976 B/op    642 allocs/op
// leaves=1000-16    220609 ns/op  570938 B/op   6529 allocs/op
// leaves=10000-16  2209920 ns/op 5649435 B/op  69557 allocs/op
func BenchmarkParseRawInput(b *testing.B) {
	r := &Rego{}
	for _, n := range []int{10, 100, 1000, 10000} {
		input := benchNativeInputTree(n)

		b.Run(fmt.Sprintf("leaves=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				v := any(input)
				if _, err := r.parseRawInput(&v, metrics.New()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchNativeInputTree(n int) map[string]any {
	arr := make([]any, 0, n/2)
	obj := make(map[string]any, n/2)
	for i := range n {
		if i%2 == 0 {
			arr = append(arr, map[string]any{"i": json.Number(strconv.Itoa(i)), "s": "value"})
		} else {
			obj[fmt.Sprintf("key%d", i)] = json.Number(strconv.Itoa(i))
		}
	}
	return map[string]any{"arr": arr, "obj": obj}
}
