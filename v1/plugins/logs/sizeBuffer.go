package logs

import (
	"context"
	"encoding/json"
	"math"
	"sync"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"golang.org/x/time/rate"
)

type sizeBuffer struct {
	mtx                  sync.Mutex
	buffer               *logBuffer
	BufferSizeLimitBytes int64
	UploadSizeLimitBytes int64
	enc                  *chunkEncoder // encoder appends events into the gzip compressed JSON array
	limiter              *rate.Limiter
	metrics              metrics.Metrics
	logger               logging.Logger
	client               rest.Client
	uploadPath           string
	mode                 plugins.TriggerMode
	resetTimer           chan struct{}
}

func newSizeBuffer(
	bufferSizeLimitBytes int64,
	uploadSizeLimitBytes int64,
	client rest.Client,
	uploadPath string,
	mode plugins.TriggerMode,
	resetTimer chan struct{}) *sizeBuffer {
	return &sizeBuffer{
		enc:                  newChunkEncoder(uploadSizeLimitBytes),
		buffer:               newLogBuffer(bufferSizeLimitBytes),
		BufferSizeLimitBytes: uploadSizeLimitBytes,
		UploadSizeLimitBytes: uploadSizeLimitBytes,
		client:               client,
		uploadPath:           uploadPath,
		mode:                 mode,
		resetTimer:           resetTimer,
	}
}

func (b *sizeBuffer) WithLimiter(maxDecisionsPerSecond *float64) *sizeBuffer {
	if maxDecisionsPerSecond != nil {
		b.limiter = rate.NewLimiter(rate.Limit(*maxDecisionsPerSecond), int(math.Max(1, *maxDecisionsPerSecond)))
	}
	return b
}

func (b *sizeBuffer) WithMetrics(m metrics.Metrics) {
	b.metrics = m
	b.enc.metrics = m
}

func (b *sizeBuffer) WithLogger(l logging.Logger) *sizeBuffer {
	b.logger = l
	b.enc.logger = l
	return b
}

func (*sizeBuffer) Stop() {}

func (*sizeBuffer) Name() string {
	return sizeBufferType
}

func (b *sizeBuffer) incrMetric(name string) {
	if b.metrics != nil {
		b.metrics.Counter(name).Incr()
	}
}

func (b *sizeBuffer) Reconfigure(
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

	// The encoder and buffer will use these new values after upload
	b.UploadSizeLimitBytes = uploadSizeLimitBytes
	b.BufferSizeLimitBytes = bufferSizeLimitBytes

	b.mode = mode
	b.client = client
	b.uploadPath = uploadPath
}

func (b *sizeBuffer) Push(event *EventV1) {
	if b.limiter != nil && !b.limiter.Allow() {
		b.incrMetric(logRateLimitExDropCounterName)
		b.logger.Error("Decision log dropped as rate limit exceeded. Reduce reporting interval or increase rate limit.")
		return
	}

	eventBytes, err := json.Marshal(&event)
	if err != nil {
		b.logger.Error("Decision log dropped due to error serializing event to JSON: %v", err)
		return
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()
	result, err := b.enc.Encode(*event, eventBytes)
	if err != nil {
		return
	}

	if result == nil {
		return
	}

	switch b.mode {
	case plugins.TriggerImmediate:
		// we have a result, reset timer to attempt upload
		b.resetTimer <- struct{}{}

		ctx := context.Background()
		for _, chunk := range result {
			err = b.uploadChunk(ctx, chunk, err)
		}
		// flush any failed to upload chunks
		for bs := b.buffer.Pop(); bs != nil; bs = b.buffer.Pop() {
			err = b.uploadChunk(ctx, bs, err)
		}
	case plugins.TriggerPeriodic:
		for _, chunk := range result {
			b.bufferChunk(b.buffer, chunk)
		}
	}
}

func (b *sizeBuffer) Upload(ctx context.Context) error {
	// Make a local copy of the plugin's encoder and buffer and create
	// a new encoder and buffer. This is needed as locking the buffer for
	// the upload duration will block policy evaluation and result in
	// increased latency for OPA clients
	b.mtx.Lock()
	if b.mode == plugins.TriggerImmediate {
		select {
		case <-b.resetTimer:
			// This catches the scenario if the timer loop expired while the read() loop was uploading a chunk
			// The timer loop won't be able to read the reset timer because it will be waiting for doOneShot
			// Just return because then the timer will be reset
			return nil
		default:
		}
	}

	oldChunkEnc := b.enc
	oldBuffer := b.buffer
	b.buffer = newLogBuffer(b.BufferSizeLimitBytes)
	b.enc = newChunkEncoder(b.UploadSizeLimitBytes).WithMetrics(b.metrics).WithLogger(b.logger).
		WithUncompressedLimit(oldChunkEnc.uncompressedLimit, oldChunkEnc.uncompressedLimitScaleDownExponent, oldChunkEnc.uncompressedLimitScaleUpExponent)
	b.mtx.Unlock()

	// Along with uploading the compressed events in the buffer
	// to the remote server, flush any pending compressed data to the
	// underlying writer and add to the buffer.
	chunk, err := oldChunkEnc.Flush()
	if err != nil {
		return err
	}

	for _, ch := range chunk {
		b.bufferChunk(oldBuffer, ch)
	}

	if oldBuffer.Len() == 0 {
		return &bufferEmpty{}
	}

	for bs := oldBuffer.Pop(); bs != nil; bs = oldBuffer.Pop() {
		err = b.uploadChunk(ctx, bs, err)
	}

	return err
}

func (b *sizeBuffer) uploadChunk(ctx context.Context, chunk []byte, err error) error {
	if err == nil {
		err = uploadChunk(ctx, b.client, b.uploadPath, chunk)
	}
	if err != nil {
		if b.limiter != nil {
			events, decErr := newChunkDecoder(chunk).decode()
			if decErr != nil {
				return err
			}

			for i := range events {
				b.Push(&events[i])
			}
		} else {
			// requeue the chunk
			b.mtx.Lock()
			b.bufferChunk(b.buffer, chunk)
			b.mtx.Unlock()
		}
	}
	return err
}

func (b *sizeBuffer) bufferChunk(buffer *logBuffer, bs []byte) {
	dropped := buffer.Push(bs)
	if dropped > 0 {
		b.incrMetric(logBufferEventDropCounterName)
		b.incrMetric(logBufferSizeLimitExDropCounterName)
		b.logger.Error("Dropped %v chunks from buffer. Reduce reporting interval or increase buffer size.", dropped)
	}
}
