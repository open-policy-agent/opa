// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime/metrics"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/file/archive"
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

func BenchmarkReadManyModules(b *testing.B) {
	files := [][2]string{{"/.manifest", `{"rego_version": 1}`}}
	for f := range 160 {
		var sb strings.Builder
		sb.WriteString("package gen\n\n")
		for r := f * 500; r < (f+1)*500; r++ {
			fmt.Fprintf(&sb, "# METADATA\n# labels:\n#   id: \"r%d\"\nallow if {\n\tinput.action == \"read%d\"\n\tinput.role == \"viewer%d\"\n\tinput.resource.id == \"res-%d\"\n}\n\n", r, r%17, r%101, r)
		}
		files = append(files, [2]string{fmt.Sprintf("/gen/rules_%d.rego", f), sb.String()})
	}
	tb := archive.MustWriteTarGz(files).Bytes()

	sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	var peak atomic.Uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
				metrics.Read(sample)
				if v := sample[0].Value.Uint64(); v > peak.Load() {
					peak.Store(v)
				}
			}
		}
	}()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewCustomReader(NewTarballLoaderWithBaseURL(bytes.NewReader(tb), "")).WithProcessAnnotations(true).Read(); err != nil {
			b.Fatal(err)
		}
	}
	close(stop)
	<-done
	b.ReportMetric(float64(peak.Load()), "peak-heap-B")
}
