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
	cancelUpload         bool
}

func newSizeBuffer(
	bufferSizeLimitBytes int64,
	uploadSizeLimitBytes int64,
	client rest.Client,
	uploadPath string,
	mode plugins.TriggerMode) *sizeBuffer {
	return &sizeBuffer{
		enc:                  newChunkEncoder(uploadSizeLimitBytes),
		buffer:               newLogBuffer(bufferSizeLimitBytes),
		BufferSizeLimitBytes: uploadSizeLimitBytes,
		UploadSizeLimitBytes: uploadSizeLimitBytes,
		client:               client,
		uploadPath:           uploadPath,
		mode:                 mode,
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

	b.UploadSizeLimitBytes = uploadSizeLimitBytes
	b.enc.Reconfigure(uploadSizeLimitBytes)
	b.BufferSizeLimitBytes = bufferSizeLimitBytes
	b.buffer.Reconfigure(bufferSizeLimitBytes)

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
		ctx := context.Background()

		var uploadErr error
		for _, chunk := range result {
			uploadErr = uploadChunk(ctx, b.client, b.uploadPath, chunk)
			if uploadErr != nil {
				break
			}
		}

		if uploadErr != nil {
			for _, chunk := range result {
				b.bufferChunk(b.buffer, chunk)
			}
		} else {
			for bs := b.buffer.Pop(); bs != nil; bs = b.buffer.Pop() {
				uploadErr = uploadChunk(ctx, b.client, b.uploadPath, bs)
				if uploadErr != nil {
					b.bufferChunk(b.buffer, bs)
					break
				}
			}
		}

		if uploadErr != nil {
			b.cancelUpload = true
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
		if b.cancelUpload {
			b.cancelUpload = false
			return nil
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
		if err == nil {
			err = uploadChunk(ctx, b.client, b.uploadPath, bs)
		}
		if err != nil {
			if b.limiter != nil {
				events, decErr := newChunkDecoder(bs).decode()
				if decErr != nil {
					return err
				}

				for i := range events {
					b.Push(&events[i])
				}
			} else {
				// requeue the chunk
				b.mtx.Lock()
				b.bufferChunk(b.buffer, bs)
				b.mtx.Unlock()
			}
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
