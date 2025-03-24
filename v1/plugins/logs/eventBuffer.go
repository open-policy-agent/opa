// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"context"
	"encoding/json"
	"fmt"
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
func (b *eventBuffer) Reconfigure(ctx context.Context, bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) {
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
		b.push(ctx, event)
	}
}

// Push attempts to add a new event to the buffer, returning true if an event was dropped.
// This can be called concurrently.
func (b *eventBuffer) Push(ctx context.Context, event *EventV1) {
	b.push(ctx, bufferItem{EventV1: event})
}

func (b *eventBuffer) push(ctx context.Context, event bufferItem) {
	maxEventRetry := 1000

	for range maxEventRetry {
		select {
		case <-ctx.Done():
			return
		case b.buffer <- event:
			return
		default:
			<-b.buffer
			b.incrMetric(logBufferEventDropCounterName)
		}
	}

}

// Upload reads all the currently buffered events and uploads them as a gzip compressed JSON array to the service.
// The events are uploaded as chunks limited by the uploadSizeLimitBytes.
func (b *eventBuffer) Upload(ctx context.Context) (bool, error) {
	b.upload.Lock()
	defer b.upload.Unlock()

	nextEvent := b.compress()

	if b.chunk.bytesWritten == 0 {
		return false, nil
	}

	currentChunk := b.chunk.buf.Bytes()
	if err := uploadChunk(ctx, b.client, b.uploadPath, currentChunk); err != nil {
		b.chunk.initialize()

		if nextEvent != nil {
			// upload failed, the next chunk can't be made yet so push event back into the buffer.
			b.push(ctx, *nextEvent)
		}

		events, decErr := newChunkDecoder(currentChunk).decode()
		if decErr != nil {
			return false, fmt.Errorf("%w: %w", err, decErr)
		}

		for i := range events {
			b.Push(ctx, &events[i])
		}

		return false, err
	}

	b.chunk.initialize()
	if nextEvent != nil {
		if _, err := b.chunk.WriteBytes(nextEvent.serialized); err != nil {
			// corrupted payload, record failure and reset
			b.chunk.initialize()
			b.incrMetric(logEncodingFailureCounterName)
			b.logger.Error("encoding failure: %v, dropping event with decision ID: %v.", err, nextEvent.EventV1.DecisionID)
		}
	}

	return true, nil
}

// compress will read events from the buffer creating a gzipped compressed JSON array for upload.
// events are dropped if it exceeds the upload size limit or there is a marshalling problem.
func (b *eventBuffer) compress() *bufferItem {
	var nextEvent *bufferItem

	eventLen := len(b.buffer)
	if eventLen == 0 {
		return nil
	}

	// read until either the buffer is empty or the chunk has reached the upload limit
	for {
		event := b.readEvent()
		if event == nil {
			break
		}

		if err := b.processEvent(event); err != nil {
			b.logger.Error("%v", err)
		}

		if int64(eventLen+b.chunk.bytesWritten+1) > b.uploadSizeLimitBytes {
			// save event that doesn't fit in the current chunk to add it to the next chunk
			nextEvent = event
			break
		}

		if _, err := b.chunk.WriteBytes(event.serialized); err != nil {
			// corrupted payload, record failure and reset
			b.chunk.initialize()
			b.incrMetric(logEncodingFailureCounterName)
			b.logger.Error("encoding failure: %v, dropping event with decision ID: %v.", err, event.EventV1.DecisionID)
			continue
		}
	}

	if err := b.chunk.writeClose(); err != nil {
		// corrupted payload, record failure and reset
		b.chunk.initialize()
		b.incrMetric(logEncodingFailureCounterName)
		b.logger.Error("encoding failure: %v, dropping multiple events.")
	}

	return nextEvent
}

// readEvent does a nonblocking read from the event buffer
func (b *eventBuffer) readEvent() *bufferItem {
	select {
	case event := <-b.buffer:
		return &event
	default:
		return nil
	}
}

// processEvent serializes the event and determines if the ND cache needs to be dropped
func (b *eventBuffer) processEvent(event *bufferItem) error {
	// an event could already have been serialized if an upload reached the limit or failed
	if event.serialized == nil {
		var err error
		event.serialized, err = json.Marshal(event.EventV1)
		if err != nil {
			b.incrMetric(logEncodingFailureCounterName)
			return fmt.Errorf("encoding failure: %w, dropping event with decision ID: %v", err, event.EventV1.DecisionID)
		}
	}

	if int64(len(event.serialized)) >= b.uploadSizeLimitBytes {
		if event.NDBuiltinCache == nil {
			return fmt.Errorf("upload event size (%d) exceeds upload_size_limit_bytes (%d), dropping event with decision ID: %v",
				int64(len(event.serialized)), b.uploadSizeLimitBytes, event.EventV1.DecisionID)
		}

		// Attempt to drop the ND cache to reduce size. If it is still too large, drop the event.
		event.NDBuiltinCache = nil
		var err error
		event.serialized, err = json.Marshal(event.EventV1)
		if err != nil {
			b.incrMetric(logEncodingFailureCounterName)
			return fmt.Errorf("encoding failure: %v, dropping event with decision ID: %v", err, event.EventV1.DecisionID)
		}
		if int64(len(event.serialized)) > b.uploadSizeLimitBytes {
			return fmt.Errorf("upload event size (%d) exceeds upload_size_limit_bytes (%d), dropping event with decision ID: %v",
				int64(len(event.serialized)), b.uploadSizeLimitBytes, event.EventV1.DecisionID)
		}

		b.incrMetric(logNDBDropCounterName)
		b.logger.Error("ND builtins cache dropped from this event to fit under maximum upload size limits. Increase upload size limit or change usage of non-deterministic builtins.")
	}

	return nil
}
