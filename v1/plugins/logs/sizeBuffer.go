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
	mtx        sync.Mutex
	uploadMtx  sync.Mutex // used only in immediate upload mode
	buffer     *logBuffer
	enc        *chunkEncoder // encoder appends events into the gzip compressed JSON array
	limiter    *rate.Limiter
	metrics    metrics.Metrics
	logger     logging.Logger
	client     rest.Client
	uploadPath string
	mode       plugins.TriggerMode
}

func newSizeBuffer(bufferSizeLimitBytes int64, uploadSizeLimitBytes int64, client rest.Client, uploadPath string, mode plugins.TriggerMode) *sizeBuffer {
	return &sizeBuffer{
		enc:        newChunkEncoder(uploadSizeLimitBytes),
		buffer:     newLogBuffer(bufferSizeLimitBytes),
		client:     client,
		uploadPath: uploadPath,
		mode:       mode,
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

func (b *sizeBuffer) Flush() []*EventV1 {
	b.uploadMtx.Lock()
	defer b.uploadMtx.Unlock()

	b.mtx.Lock()
	defer b.mtx.Unlock()

	var events []*EventV1

	chunks, err := b.enc.Flush()
	if err != nil {
		b.incrMetric(logEncodingFailureCounterName)
		if b.logger != nil {
			b.logger.Error("Decision logs dropped due to an encoding failure.")
		}
	}

	for _, chunk := range chunks {
		b.buffer.Push(chunk)
	}

	if b.buffer.Len() == 0 {
		return events
	}

	for bs := b.buffer.Pop(); bs != nil; bs = b.buffer.Pop() {
		decodedEvents, err := newChunkDecoder(bs).decode()
		if err != nil {
			b.incrMetric(logEncodingFailureCounterName)
			if b.logger != nil {
				b.logger.Error("Dropping multiple events due to encoding failure.")
			}
			continue
		}

		for i := range decodedEvents {
			events = append(events, &decodedEvents[i])
		}
	}

	return events
}

func (*sizeBuffer) Stop(_ context.Context) {
}

func (*sizeBuffer) Name() string {
	return sizeBufferType
}

func (b *sizeBuffer) incrMetric(name string) {
	if b.metrics != nil {
		b.metrics.Counter(name).Incr()
	}
}

func (b *sizeBuffer) Push(event *EventV1) {
	if b.limiter != nil && !b.limiter.Allow() {
		b.incrMetric(logRateLimitExDropCounterName)
		if b.logger != nil {
			b.logger.Error("Decision log dropped as rate limit exceeded. Reduce reporting interval or increase rate limit.")
		}
		return
	}

	eventBytes, err := json.Marshal(&event)
	if err != nil {
		if b.logger != nil {
			b.logger.Error("Decision log dropped due to error serializing event to JSON with decision ID %v", event.DecisionID)
		}

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
		go func() {
			b.uploadMtx.Lock()
			defer b.uploadMtx.Unlock()

			ctx := context.Background()

			var uploadErr error
			for _, chunk := range result {
				uploadErr = uploadChunk(ctx, b.client, b.uploadPath, chunk)
				if uploadErr != nil {
					b.mtx.Lock()
					b.bufferChunk(b.buffer, chunk)
					b.mtx.Unlock()
				}
			}
		}()
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

	oldChunkEnc := b.enc
	oldBuffer := b.buffer
	b.buffer = newLogBuffer(b.buffer.limit)
	b.enc = newChunkEncoder(b.enc.limit).WithMetrics(b.metrics).WithLogger(b.logger).
		WithUncompressedLimit(oldChunkEnc.uncompressedLimit, oldChunkEnc.uncompressedLimitScaleDownExponent, oldChunkEnc.uncompressedLimitScaleUpExponent)
	b.mtx.Unlock()

	b.uploadMtx.Lock()
	defer b.uploadMtx.Unlock()

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
					continue
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
		if b.logger != nil {
			b.logger.Error("Dropped %v chunks from buffer. Reduce reporting interval or increase buffer size.", dropped)
		}
	}
}
