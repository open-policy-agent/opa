// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"fmt"
	"testing"
)

func BenchmarkSelect(b *testing.B) {
	sizes := []int{1000, 10000, 100000}
	for _, size := range sizes {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			tpe := generateType(size)
			runSelectBenchmark(b, tpe, json.Number(fmt.Sprint(size-1)))
		})
	}
}

func runSelectBenchmark(b *testing.B, tpe Type, key interface{}) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if result := Select(tpe, key); result != nil {
			if Compare(result, N) != 0 {
				b.Fatal("expected number type")
			}
		}
	}
}

func generateType(n int) Type {
	static := make([]*StaticProperty, n)
	for i := 0; i < n; i++ {
		static[i] = NewStaticProperty(json.Number(fmt.Sprint(i)), N)
	}
	return NewObject(static, nil)
}
