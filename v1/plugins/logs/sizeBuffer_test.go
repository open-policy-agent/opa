package logs

import (
	"errors"
	"testing"

	"github.com/open-policy-agent/opa/v1/plugins/rest"
)

func TestSizeBuffer_Upload(t *testing.T) {
	t.Parallel()

	bufferSizeLimit := int64(100)
	UploadSizeLimit := int64(100)
	b := newSizeBuffer(bufferSizeLimit, UploadSizeLimit)

	if b.enc.limit != UploadSizeLimit {
		t.Fatalf("expected encoder limit to be %d, got %d", UploadSizeLimit, b.enc.limit)
	}
	if b.buffer.limit != bufferSizeLimit {
		t.Fatalf("expected buffer limit to be %d, got %d", bufferSizeLimit, b.buffer.limit)
	}

	err := b.Upload(t.Context(), rest.Client{}, "")
	if err == nil {
		t.Fatalf("expected error, got %s", err)
	}

	if !errors.Is(err, &bufferEmpty{}) {
		t.Fatalf("expected error, got %s", err)
	}

	if b.enc.limit != UploadSizeLimit {
		t.Fatalf("expected encoder limit to be %d, got %d", UploadSizeLimit, b.enc.limit)
	}
	if b.buffer.limit != bufferSizeLimit {
		t.Fatalf("expected buffer limit to be %d, got %d", bufferSizeLimit, b.buffer.limit)
	}

	newUploadSizeLimit := int64(200)
	newBufferSizeLimit := int64(200)

	b.Reconfigure(newBufferSizeLimit, newUploadSizeLimit, nil)

	if b.enc.limit != newUploadSizeLimit {
		t.Fatalf("expected encoder limit to be %d, got %d", newUploadSizeLimit, b.enc.limit)
	}
	if b.buffer.limit != newBufferSizeLimit {
		t.Fatalf("expected buffer limit to be %d, got %d", newBufferSizeLimit, b.buffer.limit)
	}
}
