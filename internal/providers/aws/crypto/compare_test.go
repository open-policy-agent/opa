package crypto

import (
	"bytes"
	"math/big"
	"testing"
)

func TestConstantTimeByteCompare(t *testing.T) {
	cases := []struct {
		x, y      []byte
		r         int
		expectErr bool
	}{
		{x: []byte{}, y: []byte{}, r: 0},
		{x: []byte{40}, y: []byte{30}, r: 1},
		{x: []byte{30}, y: []byte{40}, r: -1},
		{x: []byte{60, 40, 30, 10, 20}, y: []byte{50, 30, 20, 0, 10}, r: 1},
		{x: []byte{50, 30, 20, 0, 10}, y: []byte{60, 40, 30, 10, 20}, r: -1},
		{x: nil, y: []byte{}, r: 0},
		{x: []byte{}, y: nil, r: 0},
		{x: []byte{}, y: []byte{10}, expectErr: true},
		{x: []byte{10}, y: []byte{}, expectErr: true},
		{x: []byte{10, 20}, y: []byte{10}, expectErr: true},
	}

	for _, tt := range cases {
		compare, err := ConstantTimeByteCompare(tt.x, tt.y)
		if (err != nil) != tt.expectErr {
			t.Fatalf("expectErr=%v, got %v", tt.expectErr, err)
		}
		if e, a := tt.r, compare; e != a {
			t.Errorf("expect %v, got %v", e, a)
		}
	}
}

func BenchmarkConstantTimeCompare(b *testing.B) {
	x, y := big.NewInt(1023), big.NewInt(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ConstantTimeByteCompare(x.Bytes(), y.Bytes())
	}
}

func BenchmarkCompare(b *testing.B) {
	x, y := big.NewInt(1023).Bytes(), big.NewInt(1024).Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes.Compare(x, y)
	}
}
