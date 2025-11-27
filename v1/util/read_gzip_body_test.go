package util_test

import (
	"bytes"
	"compress/gzip"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/decoding"
)

func TestReadMaybeCompressedBody(t *testing.T) {
	t.Parallel()

	exp := []byte(`{"input": {"foo": "bar"}}`)

	bb := new(bytes.Buffer)
	gz := gzip.NewWriter(bb)
	if _, err := gz.Write(exp); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	compressed := bb.Bytes()

	ctx := decoding.AddServerDecodingGzipMaxLen(decoding.AddServerDecodingMaxLen(t.Context(), 200), 200)
	req := httptest.NewRequestWithContext(ctx, "POST", "/v1/data/test", bytes.NewReader(compressed))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	got, err := util.ReadMaybeCompressedBody(req)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, exp) {
		t.Fatalf("Expected %q, got: %q", exp, got)
	}
}

func BenchmarkReadMaybeCompressedBody(b *testing.B) {
	exp := []byte(`{"input": {"foo": "bar"}}`)

	bb := new(bytes.Buffer)
	gz := gzip.NewWriter(bb)
	if _, err := gz.Write(exp); err != nil {
		b.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		b.Fatal(err)
	}
	compressed := bb.Bytes()
	ctx := decoding.AddServerDecodingGzipMaxLen(decoding.AddServerDecodingMaxLen(b.Context(), 200), 200)

	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			req := httptest.NewRequestWithContext(ctx, "POST", "/v1/data/test", bytes.NewReader(compressed))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")

			if _, err := util.ReadMaybeCompressedBody(req); err != nil {
				b.Fatal(err)
			}
		}
	})
}
