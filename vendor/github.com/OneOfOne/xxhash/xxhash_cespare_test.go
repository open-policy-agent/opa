// +build cespare

package xxhash_test

import (
	"testing"

	"github.com/cespare/xxhash"
)

func BenchmarkXXSum64Cespare(b *testing.B) {
	var bv uint64
	b.Run("Func", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bv += xxhash.Sum64(in)
		}
	})
	b.Run("Struct", func(b *testing.B) {
		h := xxhash.New()
		for i := 0; i < b.N; i++ {
			h.Write(in)
			bv += h.Sum64()
			h.Reset()
		}
	})
}

func BenchmarkXXSum64ShortCespare(b *testing.B) {
	var bv uint64
	k := []byte("Test-key-100")
	b.Run("Func", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bv += xxhash.Sum64(k)
		}
	})
	b.Run("Struct", func(b *testing.B) {
		h := xxhash.New()
		for i := 0; i < b.N; i++ {
			h.Write(k)
			bv += h.Sum64()
			h.Reset()
		}
	})
}
