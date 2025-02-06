package errors

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/storage"
)

// 160.8 ns/op    96 B/op    5 allocs/op    // using fmt.Sprintf
// 58.31 ns/op     8 B/op    1 allocs/op    // using string concatenation
// ...
func BenchmarkNewNotFoundErrorWithHint(b *testing.B) {
	path := storage.Path([]string{"a", "b", "c"})
	hint := "something something"

	for range b.N {
		err := NewNotFoundErrorWithHint(path, hint)
		if err == nil {
			b.Fatal("expected error")
		}
	}
}
