// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
)

// BenchmarkBundleCopy exercises Bundle.Copy's deep copy of nested bundle data.
func BenchmarkBundleCopy(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 10000} {
		bundle := Bundle{Data: benchTestNestedData(n)}
		bundle.Manifest.Init()

		b.Run(fmt.Sprintf("leaves=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = bundle.Copy()
			}
		})
	}
}

func benchTestNestedData(n int) map[string]any {
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
