package util_test

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/decoding"
)

func TestReadMaybeCompressedBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		payload       []byte
		forgedSize    uint32 // If > 0, overwrite trailer with this size
		limit         int64
		expectedError string
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
			expectedError: "request body too large: decompressed size exceeds limit of 200 bytes (server.decoding.gzip.max_length)",
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

func TestReadMaybeCompressedBodyExceedsMaxLength(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("a"), 20)
	ctx := decoding.AddServerDecodingMaxLen(t.Context(), 10)
	req := httptest.NewRequestWithContext(ctx, "POST", "/v1/data/test", io.NopCloser(bytes.NewReader(body)))
	req.ContentLength = -1 // chunked

	_, err := util.ReadMaybeCompressedBody(req)

	if _, ok := errors.AsType[*util.BodyTooLargeError](err); !ok {
		t.Fatalf("Expected BodyTooLargeError, got: %v", err)
	}
	exp := "request body too large: exceeds limit of 10 bytes (server.decoding.max_length)"
	if err.Error() != exp {
		t.Fatalf("Expected error %q, got: %v", exp, err)
	}
}

func TestReadMaybeCompressedBodyMaxInt64Limit(t *testing.T) {
	t.Parallel()

	body := []byte(`{"input": 1}`)
	ctx := decoding.AddServerDecodingMaxLen(t.Context(), math.MaxInt64)
	req := httptest.NewRequestWithContext(ctx, "POST", "/v1/data/test", io.NopCloser(bytes.NewReader(body)))
	req.ContentLength = -1 // chunked

	got, err := util.ReadMaybeCompressedBody(req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("Expected %q, got: %q", body, got)
	}
}

func TestBodyTooLargeErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note string
		err  util.BodyTooLargeError
		exp  string
	}{
		{
			note: "unknown size, limit in bytes",
			err:  util.BodyTooLargeError{Limit: 256 << 20},
			exp:  "request body too large: exceeds limit of 268435456 bytes (server.decoding.max_length)",
		},
		{
			note: "size under 1 KiB stays in bytes",
			err:  util.BodyTooLargeError{Limit: 100, Size: 512},
			exp:  "request body too large: 512 bytes exceeds limit of 100 bytes (server.decoding.max_length)",
		},
		{
			note: "size in MiB",
			err:  util.BodyTooLargeError{Limit: 256 << 20, Size: 300_000_000},
			exp:  "request body too large: 286.1 MiB exceeds limit of 268435456 bytes (server.decoding.max_length)",
		},
		{
			note: "exact unit has no decimal",
			err:  util.BodyTooLargeError{Limit: 1024, Size: 3 << 30},
			exp:  "request body too large: 3 GiB exceeds limit of 1024 bytes (server.decoding.max_length)",
		},
		{
			note: "decompressed",
			err:  util.BodyTooLargeError{Limit: 10, Decompressed: true},
			exp:  "request body too large: decompressed size exceeds limit of 10 bytes (server.decoding.gzip.max_length)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.exp {
				t.Fatalf("Expected %q, got: %q", tc.exp, got)
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
