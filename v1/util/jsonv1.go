//go:build !go1.27

package util

import (
	"bytes"
)

type stringOrBytes interface {
	string | []byte
}

// JsonEqual
func JsonEqual[A, B stringOrBytes](a A, b B) bool {
	v1, v2 := []byte(a), []byte(b)

	return bytes.Equal(v1, v2)
}
