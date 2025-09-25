// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"context"
	"encoding/json"
	"math"
	"sync"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/util"
	"golang.org/x/time/rate"
)

type bufferItem struct {
	*EventV1        // an individual event
	chunk    []byte // a ready to upload compressed JSON Array of events
}

// eventBuffer stores and uploads a gzip compressed JSON array of EventV1 entries
type eventBuffer struct {
	buffer  chan *bufferItem // buffer stores JSON encoded EventV1 data
	upload  sync.Mutex       // upload controls that uploads are done sequentially
	enc     *chunkEncoder    // encoder appends events into the gzip compressed JSON array
	limiter *rate.Limiter
	metrics metrics.Metrics
	logger  logging.Logger
}

func newEventBuffer(bufferSizeLimitEvents int64, uploadSizeLimitBytes int64) *eventBuffer {
	return &eventBuffer{
		buffer: make(chan *bufferItem, bufferSizeLimitEvents),
		enc:    newChunkEncoder(uploadSizeLimitBytes),
	}
}

func (b *eventBuffer) WithLimiter(maxDecisionsPerSecond *float64) *eventBuffer {
	if maxDecisionsPerSecond != nil {
		b.limiter = rate.NewLimiter(rate.Limit(*maxDecisionsPerSecond), int(math.Max(1, *maxDecisionsPerSecond)))
	}
	return b
}

func (b *eventBuffer) WithMetrics(m metrics.Metrics) {
	b.metrics = m
	b.enc = b.enc.WithMetrics(m)
}

func (*eventBuffer) Name() string {
	return eventBufferType
}

func (b *eventBuffer) WithLogger(l logging.Logger) *eventBuffer {
	b.logger = l
	b.enc = b.enc.WithLogger(l)
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
func (b *eventBuffer) Reconfigure(bufferSizeLimitEvents int64, uploadSizeLimitBytes int64, maxDecisionsPerSecond *float64) {
	// prevent an upload from pushing events that failed to upload back into a closed buffer
	b.upload.Lock()
	defer b.upload.Unlock()

	if maxDecisionsPerSecond != nil {
		b.limiter = rate.NewLimiter(rate.Limit(*maxDecisionsPerSecond), int(math.Max(1, *maxDecisionsPerSecond)))
	} else if b.limiter != nil {
		b.limiter = nil
	}

	if b.enc.limit != uploadSizeLimitBytes {
		b.enc.Reconfigure(uploadSizeLimitBytes)
	}

	if int64(cap(b.buffer)) == bufferSizeLimitEvents {
		return
	}

	close(b.buffer)
	oldBuffer := b.buffer
	b.buffer = make(chan *bufferItem, bufferSizeLimitEvents)

	for event := range oldBuffer {
		b.push(event)
	}
}

// Push attempts to add a new event to the buffer, returning true if an event was dropped.
// This can be called concurrently.
func (b *eventBuffer) Push(event *EventV1) {
	b.push(&bufferItem{EventV1: event})
}

func (b *eventBuffer) push(event *bufferItem) {
	if b.limiter != nil && !b.limiter.Allow() {
		b.incrMetric(logRateLimitExDropCounterName)
		b.logger.Error("Decision log dropped as rate limit exceeded. Reduce reporting interval or increase rate limit.")
		return
	}

	util.PushFIFO(b.buffer, event, b.metrics, logBufferEventDropCounterName)
}

// Upload reads events from the buffer and uploads them to the configured client.
// All the events currently in the buffer are read and written to a gzip compressed JSON array to create a chunk of data.
// Each chunk is limited by the uploadSizeLimitBytes.
func (b *eventBuffer) Upload(ctx context.Context, client rest.Client, uploadPath string) error {
	b.upload.Lock()
	defer b.upload.Unlock()

	eventLen := len(b.buffer)
	if eventLen == 0 {
		return &bufferEmpty{}
	}

	for range eventLen {
		bufItem := b.readBufItem()
		if bufItem == nil {
			break
		}

		var result [][]byte
		if bufItem.chunk != nil {
			result = [][]byte{bufItem.chunk}
		} else {
			event := bufItem.EventV1
			eventBytes, err := json.Marshal(&event)
			if err != nil {
				return err
			}

			result, err = b.enc.Encode(*event, eventBytes)
			if err != nil {
				b.incrMetric(logEncodingFailureCounterName)
				if b.logger != nil {
					b.logger.Error("encoding failure: %v, dropping event with decision ID: %v", err, event.DecisionID)
				}
			}
		}

		if err := b.uploadChunks(ctx, result, client, uploadPath); err != nil {
			return err
		}
	}

	// flush any chunks that didn't hit the upload limit
	result, err := b.enc.Flush()
	if err != nil {
		b.incrMetric(logEncodingFailureCounterName)
		if b.logger != nil {
			b.logger.Error("encoding failure: %v", err)
		}
		return nil
	}

	if err := b.uploadChunks(ctx, result, client, uploadPath); err != nil {
		return err
	}

	return nil
}

// uploadChunks attempts to upload multiple chunks to the configured client.
// In case of failure all the events are added back to the buffer.
func (b *eventBuffer) uploadChunks(ctx context.Context, result [][]byte, client rest.Client, uploadPath string) error {
	var finalErr error
	for _, chunk := range result {
		err := uploadChunk(ctx, client, uploadPath, chunk)

		// if an upload failed, requeue the chunk
		if err != nil {
			finalErr = err
			b.push(&bufferItem{chunk: chunk})
		}
	}
	return finalErr
}

// readEvent does a nonblocking read from the event buffer
func (b *eventBuffer) readBufItem() *bufferItem {
	select {
	case event := <-b.buffer:
		return event
	default:
		return nil
	}
}
