package logs

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
)

// bufferItem is an extended EventV1 to also hold the serialized version to avoid re-serialization.
type bufferItem struct {
	*EventV1
	serialized []byte
}

// eventBuffer stores and uploads a gzip compressed JSON array of EventV1 entries
type eventBuffer struct {
	buffer               chan bufferItem // buffer stores JSON encoded EventV1 data
	chunk                *chunkEncoder   // chunk contains the compressed JSON array to be uploaded
	upload               sync.Mutex      // upload controls that uploads are done sequentially
	client               rest.Client     // client is used to upload the data to the configured service
	uploadPath           string          // uploadPath is the configured HTTP resource path for upload
	uploadSizeLimitBytes int64           // uploadSizeLimitBytes will enforce a maximum payload size to be uploaded
	metrics              metrics.Metrics
	logger               logging.Logger
}

func newEventBuffer(bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) *eventBuffer {
	return &eventBuffer{
		buffer:               make(chan bufferItem, bufferSizeLimitEvents),
		chunk:                newChunkEncoder(uploadSizeLimitBytes),
		client:               client,
		uploadPath:           uploadPath,
		uploadSizeLimitBytes: uploadSizeLimitBytes,
	}
}

func (b *eventBuffer) WithMetrics(m metrics.Metrics) *eventBuffer {
	b.metrics = m
	return b
}

func (b *eventBuffer) WithLogger(l logging.Logger) *eventBuffer {
	b.logger = l
	return b
}

func (b *eventBuffer) incrMetric(name string) {
	if b.metrics != nil {
		b.metrics.Counter(name).Incr()
	}
}

// Reconfigure updates the user configurable values
// This cannot be called concurrently, this could change the underlying channel.
// Plugin manages a lock to control this so that changes to both buffer types can be managed sequentially.
func (b *eventBuffer) Reconfigure(bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) {
	b.client = client
	b.uploadPath = uploadPath
	b.uploadSizeLimitBytes = uploadSizeLimitBytes

	if int64(cap(b.buffer)) == bufferSizeLimitEvents {
		return
	}

	close(b.buffer)
	oldBuffer := b.buffer
	b.buffer = make(chan bufferItem, bufferSizeLimitEvents)

	for event := range oldBuffer {
		b.Push(event)
	}
}

// Push attempts to add a new event to the buffer, returning true if an event was dropped.
// This can be called concurrently.
func (b *eventBuffer) Push(event bufferItem) {
	select {
	case b.buffer <- event:
	default:
		<-b.buffer
		b.buffer <- event
		b.incrMetric(logBufferEventDropCounterName)
	}
}

// Upload reads all the currently buffered events and uploads them as a gzip compressed JSON array to the service.
// The events are uploaded as chunks limited by the uploadSizeLimitBytes.
func (b *eventBuffer) Upload(ctx context.Context) (bool, error) {
	b.upload.Lock()
	defer b.upload.Unlock()

	b.compress()

	if b.chunk.bytesWritten == 0 {
		return false, nil
	}

	if err := uploadChunk(ctx, b.client, b.uploadPath, b.chunk.buf.Bytes()); err != nil {
		return false, err
	}
	b.chunk.initialize()
	return true, nil
}

// compress will read events from the buffer creating a gzipped compressed JSON array for upload.
// events are dropped if it exceeds the upload size limit or there is a marshalling problem.
func (b *eventBuffer) compress() {
	// previous payload hasn't been sent and needs to be retried
	if b.chunk.bytesWritten != 0 {
		return
	}

	eventLen := len(b.buffer)
	if eventLen == 0 {
		return
	}

	// Uploads at most the capacity of the buffer (buffer_size_limit_events)
	for range eventLen {
		select {
		case event := <-b.buffer:
			// an event could already have been serialized if an upload reached the limit or failed
			if event.serialized == nil {
				var err error
				event.serialized, err = json.Marshal(event.EventV1)
				if err != nil {
					b.incrMetric(logEncodingFailureCounterName)
					b.logger.Error("encoding failure: %v, dropping event with decision ID: %v.", err, event.EventV1.DecisionID)
					continue
				}
			}

			if int64(len(event.serialized)) >= b.uploadSizeLimitBytes {
				if event.NDBuiltinCache == nil {
					b.logger.Error("upload event size (%d) exceeds upload_size_limit_bytes (%d), dropping event with decision ID: %v.",
						int64(len(event.serialized)), b.uploadSizeLimitBytes, event.EventV1.DecisionID)
					continue
				}

				// Attempt to drop the ND cache to reduce size. If it is still too large, drop the event.
				event.NDBuiltinCache = nil
				var err error
				event.serialized, err = json.Marshal(event.EventV1)
				if err != nil {
					b.incrMetric(logEncodingFailureCounterName)
					b.logger.Error("encoding failure: %v, dropping event with decision ID: %v.", err, event.EventV1.DecisionID)
					continue
				}
				if int64(len(event.serialized)) > b.uploadSizeLimitBytes {
					b.logger.Error("upload event size (%d) exceeds upload_size_limit_bytes (%d), dropping event with decision ID: %v.",
						int64(len(event.serialized)), b.uploadSizeLimitBytes, event.EventV1.DecisionID)
					continue
				}

				b.incrMetric(logNDBDropCounterName)
				b.logger.Error("ND builtins cache dropped from this event to fit under maximum upload size limits. Increase upload size limit or change usage of non-deterministic builtins.")
			}

			if int64(eventLen+b.chunk.bytesWritten+1) > b.uploadSizeLimitBytes {
				// add the event that doesn't fit back into the buffer
				b.Push(event)
				break
			}

			if _, err := b.chunk.WriteBytes(event.serialized); err != nil {
				// corrupted payload, record failure and reset
				b.chunk.initialize()
				b.incrMetric(logEncodingFailureCounterName)
				b.logger.Error("encoding failure: %v, dropping event with decision ID: %v.", err, event.EventV1.DecisionID)
				continue
			}
		default:
			break
		}
	}

	if err := b.chunk.writeClose(); err != nil {
		// corrupted payload, record failure and reset
		b.chunk.initialize()
		b.incrMetric(logEncodingFailureCounterName)
		b.logger.Error("encoding failure: %v, dropping multiple events.")
		return
	}
}
