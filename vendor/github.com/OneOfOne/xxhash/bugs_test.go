// +build ignore

package xxhash_test

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"testing/quick"

	N "github.com/OneOfOne/xxhash"
)

func TestReset64(t *testing.T) {
	h := N.New64()

	//
	p1 := "http"
	p2 := "://"
	p3 := "www.marmiton.org"
	p4 := "/recettes/recherche.aspx"
	p5 := "?st=2&aqt=gateau&"

	url := p1 + p2 + p3 + p4 + p5

	// compute hash by parts
	h.Write([]byte(p1))
	h.Write([]byte(p2))
	h.Write([]byte(p3))
	h.Write([]byte(p4))
	h.Write([]byte(p5))
	s1 := h.Sum64()

	h.Reset()
	h.Write([]byte(url))
	s2 := h.Sum64()

	// should be the same, right ?
	if s1 != s2 {
		t.Errorf("s1 != s2 %x %x", s1, s2)
	}
}

func TestReset32(t *testing.T) {
	h := N.New32()

	//
	p1 := "http"
	p2 := "://"
	p3 := "www.marmiton.org"
	p4 := "/recettes/recherche.aspx"
	p5 := "?st=2&aqt=gateau&"

	url := p1 + p2 + p3 + p4 + p5

	// compute hash by parts
	h.Write([]byte(p1))
	h.Write([]byte(p2))
	h.Write([]byte(p3))
	h.Write([]byte(p4))
	h.Write([]byte(p5))
	s1 := h.Sum32()

	h.Reset()
	h.Write([]byte(url))
	s2 := h.Sum32()

	// should be the same, right ?
	if s1 != s2 {
		t.Errorf("s1 != s2 %x %x", s1, s2)
	}
}

// issue 8
func TestDataLen(t *testing.T) {
	for i := 4; i <= 8096; i += 4 {
		testEquality(t, bytes.Repeat([]byte("www."), i/4))
	}
}

func testEquality(t *testing.T, v []byte) {
	ch64, ch32 := cxx.Checksum64(v), cxx.Checksum32(v)

	if h := N.Checksum64(v); ch64 != h {
		t.Fatalf("Checksum64 doesn't match, len = %d, expected 0x%X, got 0x%X", len(v), ch64, h)
	}

	if h := N.Checksum32(v); ch32 != h {
		t.Fatalf("Checksum32 doesn't match, len = %d, expected 0x%X, got 0x%X", len(v), ch32, h)
	}

	h64 := N.New64()
	h64.Write(v)

	if h := h64.Sum64(); ch64 != h {
		t.Fatalf("Sum64() doesn't match, len = %d, expected 0x%X, got 0x%X", len(v), ch64, h)
	}

	h32 := N.New32()
	h32.Write(v)

	if h := h32.Sum32(); ch32 != h {
		t.Fatalf("Sum32() doesn't match, len = %d, expected 0x%X, got 0x%X", len(v), ch32, h)
	}
}

func TestHulkSmash(t *testing.T) {
	const C = 10000
	rnd, typ := rand.New(rand.NewSource(time.Now().UnixNano())), reflect.TypeOf([]byte(nil))
	for i := 0; i < C; i++ {
		v, ok := quick.Value(typ, rnd)
		if !ok {
			t.Fatal("!ok")
		}
		vb := v.Bytes()
		seed := uint64(rnd.Int63())
		x64 := N.NewS64(seed)
		x64.Write(vb)
		if s1, s2 := x64.Sum64(), N.Checksum64S(vb, seed); s1 != s2 {
			t.Fatalf("len(v) = %d: %d != %d, should be %d", len(vb), s1, s2, cxx.Checksum64S(vb, seed))
		}
		x32 := N.NewS32(uint32(seed))
		x32.Write(vb)
		if s1, s2 := x32.Sum32(), N.Checksum32S(vb, uint32(seed)); s1 != s2 {
			t.Fatalf("len(v) = %d: %d != %d, should be %d", len(vb), s1, s2, cxx.Checksum32S(vb, uint32(seed)))
		}
	}
}
