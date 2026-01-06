package util_test

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/decoding"
)

func TestReadMaybeCompressedBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		expectedError string
		payload       []byte
		limit         int64
		forgedSize    uint32
	}{
		{
			name:          "valid payload",
			payload:       []byte(`{"input": {"foo": "bar"}}`),
			limit:         200,
			expectedError: "",
		},
		{
			name:          "forged small trailer, actual content larger",
			payload:       bytes.Repeat([]byte("a"), 100),
			forgedSize:    50,
			limit:         200,
			expectedError: "gzip: invalid checksum",
		},
		{
			name:          "content exceeds limit",
			payload:       bytes.Repeat([]byte("a"), 300),
			limit:         200,
			expectedError: "gzip payload too large",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bb := new(bytes.Buffer)
			gz := gzip.NewWriter(bb)
			if _, err := gz.Write(tc.payload); err != nil {
				t.Fatal(err)
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			compressed := bb.Bytes()

			if tc.forgedSize > 0 {
				binary.LittleEndian.PutUint32(compressed[len(compressed)-4:], tc.forgedSize)
			}

			ctx := decoding.AddServerDecodingGzipMaxLen(decoding.AddServerDecodingMaxLen(t.Context(), tc.limit), tc.limit)
			req := httptest.NewRequestWithContext(ctx, "POST", "/v1/data/test", bytes.NewReader(compressed))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")

			got, err := util.ReadMaybeCompressedBody(req)
			if tc.expectedError != "" {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if err.Error() != tc.expectedError {
					t.Fatalf("Expected error %q, got: %v", tc.expectedError, err)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if !bytes.Equal(got, tc.payload) {
					t.Fatalf("Expected %q, got: %q", tc.payload, got)
				}
			}
		})
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
