// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/util"
)

type bufferItem struct {
	*EventV1        // an individual event
	chunk    []byte // a ready to upload compressed JSON Array of events
}

// eventBuffer stores and uploads a gzip compressed JSON array of EventV1 entries
type eventBuffer struct {
	buffer               chan *bufferItem // buffer stores JSON encoded EventV1 data
	upload               sync.Mutex       // upload controls that uploads are done sequentially
	client               rest.Client      // client is used to upload the data to the configured service
	uploadPath           string           // uploadPath is the configured HTTP resource path for upload
	uploadSizeLimitBytes int64            // uploadSizeLimitBytes will enforce a maximum payload size to be uploaded
	encoder              *chunkEncoder    // encoder appends events into the gzip compressed JSON array
	metrics              metrics.Metrics
	logger               logging.Logger
}

func newEventBuffer(bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) *eventBuffer {
	return &eventBuffer{
		buffer:               make(chan *bufferItem, bufferSizeLimitEvents),
		client:               client,
		uploadPath:           uploadPath,
		uploadSizeLimitBytes: uploadSizeLimitBytes,
		encoder:              newChunkEncoder(uploadSizeLimitBytes),
	}
}

func (b *eventBuffer) WithMetrics(m metrics.Metrics) *eventBuffer {
	b.metrics = m
	b.encoder.metrics = m
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

func (b *eventBuffer) logError(fmt string, a ...interface{}) {
	if b.logger != nil {
		b.logger.Error(fmt, a)
	}
}

// Reconfigure updates the user configurable values
// This cannot be called concurrently, this could change the underlying channel.
// Plugin manages a lock to control this so that changes to both buffer types can be managed sequentially.
func (b *eventBuffer) Reconfigure(bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) {
	b.client = client
	b.uploadPath = uploadPath
	b.uploadSizeLimitBytes = uploadSizeLimitBytes
	b.encoder.limit = b.uploadSizeLimitBytes

	if int64(cap(b.buffer)) == bufferSizeLimitEvents {
		return
	}

	// prevent an upload from pushing events that failed to upload back into a closed buffer
	b.upload.Lock()
	defer b.upload.Unlock()

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
	util.PushFIFO(b.buffer, event, b.metrics, logBufferEventDropCounterName)
}

// UploadAll reads all the current events from the buffer and uploads them to the configured client.
// The events are written to a gzip compressed JSON array to create a chunk of data.
func (b *eventBuffer) UploadAll(ctx context.Context) error {
	b.upload.Lock()
	defer b.upload.Unlock()

	eventLen := len(b.buffer)
	if eventLen == 0 {
		return &bufferEmpty{}
	}

	for range eventLen {
		event := b.readEvent()
		if event == nil {
			break
		}

		if err := b.UploadChunks(ctx, b.Handle(event)); err != nil {
			return err
		}
	}

	if err := b.UploadChunks(ctx, b.Flush()); err != nil {
		return err
	}

	return nil
}

// readEvent does a nonblocking read from the event buffer
func (b *eventBuffer) readEvent() *bufferItem {
	select {
	case event := <-b.Input():
		return event
	default:
		return nil
	}
}

// Handle serializes the event and determines if the ND cache needs to be dropped
func (b *eventBuffer) Handle(event *bufferItem) [][]byte {
	if event == nil {
		return nil
	}

	if event.chunk != nil { // this is a chunk that has failed to upload and is being retried
		return [][]byte{event.chunk}
	}

	serialized, err := json.Marshal(event.EventV1)

	// The non-deterministic cache (NDBuiltinCache) could cause issues, if it is too big or can't be encoded try to drop it.
	if err != nil || int64(len(serialized)) >= b.uploadSizeLimitBytes {
		if event.NDBuiltinCache == nil {
			b.logError("upload event size (%d) exceeds upload_size_limit_bytes (%d), dropping event with decision ID: %v",
				int64(len(serialized)), b.uploadSizeLimitBytes, event.DecisionID)
		}

		// Attempt to drop the ND cache to reduce size. If it is still too large, drop the event.
		event.NDBuiltinCache = nil
		var err error
		serialized, err = json.Marshal(event.EventV1)
		if err != nil {
			b.incrMetric(logEncodingFailureCounterName)
			b.logError("encoding failure: %v, dropping event with decision ID: %v", err, event.DecisionID)
		}
		if int64(len(serialized)) > b.uploadSizeLimitBytes {
			b.logError("upload event size (%d) exceeds upload_size_limit_bytes (%d), dropping event with decision ID: %v",
				int64(len(serialized)), b.uploadSizeLimitBytes, event.DecisionID)
		}

		b.incrMetric(logNDBDropCounterName)
		b.logError("ND builtins cache dropped from this event to fit under maximum upload size limits. Increase upload size limit or change usage of non-deterministic builtins.")
	}

	result, err := b.encoder.WriteBytes(serialized)
	if err != nil {
		b.incrMetric(logEncodingFailureCounterName)
		b.logError("encoding failure: %v, dropping event with decision ID: %v", err, event.DecisionID)
		return nil
	}

	return result
}

// Input returns the buffer channel to be read from
func (b *eventBuffer) Input() <-chan *bufferItem {
	return b.buffer
}

// Flush empties the encoder and returns whatever is ready
func (b *eventBuffer) Flush() [][]byte {
	result, err := b.encoder.Flush()
	if err != nil {
		b.incrMetric(logEncodingFailureCounterName)
		b.logError("encoding failure: %v", err)
	}

	return result
}

// UploadChunks attempts to upload multiple chunks to the configured client.
// In case of failure all the events are added back to the buffer.
func (b *eventBuffer) UploadChunks(ctx context.Context, result [][]byte) error {
	var finalErr error
	for _, chunk := range result {
		err := uploadChunk(ctx, b.client, b.uploadPath, chunk)

		// if an upload failed, requeue the chunk
		if err != nil {
			finalErr = err
			b.push(&bufferItem{chunk: chunk})
		}
	}
	return finalErr
}
