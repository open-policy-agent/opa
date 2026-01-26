package logs

import (
	"errors"
	"math"
	"testing"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"golang.org/x/time/rate"
)

func TestSizeBuffer_Upload(t *testing.T) {
	t.Parallel()

	bufferSizeLimit := int64(100)
	UploadSizeLimit := int64(100)
	b := newSizeBuffer(bufferSizeLimit, UploadSizeLimit, rest.Client{}, "", plugins.TriggerPeriodic)

	if b.enc.limit != UploadSizeLimit {
		t.Fatalf("expected encoder limit to be %d, got %d", UploadSizeLimit, b.enc.limit)
	}
	if b.buffer.limit != bufferSizeLimit {
		t.Fatalf("expected buffer limit to be %d, got %d", bufferSizeLimit, b.buffer.limit)
	}

	err := b.Upload(t.Context())
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

	reconfigureSizeBuffer(b, newBufferSizeLimit, newUploadSizeLimit, nil, rest.Client{}, "", plugins.TriggerPeriodic)

	if b.enc.limit != newUploadSizeLimit {
		t.Fatalf("expected encoder limit to be %d, got %d", newUploadSizeLimit, b.enc.limit)
	}
	if b.buffer.limit != newBufferSizeLimit {
		t.Fatalf("expected buffer limit to be %d, got %d", newBufferSizeLimit, b.buffer.limit)
	}
}

func reconfigureSizeBuffer(b *sizeBuffer,
	bufferSizeLimitBytes int64,
	uploadSizeLimitBytes int64,
	maxDecisionsPerSecond *float64,
	client rest.Client,
	uploadPath string,
	mode plugins.TriggerMode) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	if maxDecisionsPerSecond != nil {
		b.limiter = rate.NewLimiter(rate.Limit(*maxDecisionsPerSecond), int(math.Max(1, *maxDecisionsPerSecond)))
	} else if b.limiter != nil {
		b.limiter = nil
	}

	b.enc.Reconfigure(uploadSizeLimitBytes)
	b.buffer.Reconfigure(bufferSizeLimitBytes)

	b.mode = mode
	b.client = client
	b.uploadPath = uploadPath
}
