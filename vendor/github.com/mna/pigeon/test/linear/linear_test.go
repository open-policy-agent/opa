package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"io"
	"testing"
	"time"
)

func TestLinearTime(t *testing.T) {
	var buf bytes.Buffer

	sizes := []int64{
		1 << 10,   // 1Kb
		10 << 10,  // 10Kb
		100 << 10, // 100Kb
		// TODO : 1Mb overflows the stack
		//1 << 20,
	}
	for _, sz := range sizes {
		r := io.LimitReader(rand.Reader, sz)
		enc := base64.NewEncoder(base64.StdEncoding, &buf)
		_, err := io.Copy(enc, r)
		if err != nil {
			t.Fatal(err)
		}
		_ = enc.Close()

		start := time.Now()
		if _, err := Parse("", buf.Bytes(), Memoize(true)); err != nil {
			t.Fatal(err)
		}
		t.Log(time.Since(start))
	}
}
