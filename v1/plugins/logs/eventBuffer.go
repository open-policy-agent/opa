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
	buffer     chan *bufferItem // buffer stores JSON encoded EventV1 data
	upload     sync.Mutex       // upload controls that uploads are done sequentially
	client     rest.Client      // client is used to upload the data to the configured service
	uploadPath string           // uploadPath is the configured HTTP resource path for upload
	enc        *chunkEncoder    // encoder appends events into the gzip compressed JSON array
	metrics    metrics.Metrics
	logger     logging.Logger
}

func newEventBuffer(bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) *eventBuffer {
	return &eventBuffer{
		buffer:     make(chan *bufferItem, bufferSizeLimitEvents),
		client:     client,
		uploadPath: uploadPath,
		enc:        newChunkEncoder(uploadSizeLimitBytes),
	}
}

func (b *eventBuffer) WithMetrics(m metrics.Metrics) *eventBuffer {
	b.metrics = m
	b.enc.metrics = m
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
	// prevent an upload from pushing events that failed to upload back into a closed buffer
	b.upload.Lock()
	defer b.upload.Unlock()

	b.client = client
	b.uploadPath = uploadPath

	if int64(cap(b.buffer)) == bufferSizeLimitEvents {
		return
	}

	b.enc.Reconfigure(uploadSizeLimitBytes)

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

// Upload reads events from the buffer and uploads them to the configured client.
// All the events currently in the buffer are read and written to a gzip compressed JSON array to create a chunk of data.
// Each chunk is limited by the uploadSizeLimitBytes.
func (b *eventBuffer) Upload(ctx context.Context) error {
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

		var result [][]byte
		if event.chunk != nil {
			result = [][]byte{event.chunk}
		} else {
			eventBytes, err := json.Marshal(&event)
			if err != nil {
				return err
			}

			result, err = b.enc.Encode(*event.EventV1, eventBytes)
			if err != nil {
				b.incrMetric(logEncodingFailureCounterName)
				if b.logger != nil {
					b.logger.Error("encoding failure: %v, dropping event with decision ID: %v", err, event.DecisionID)
				}
			}
		}

		if err := b.uploadChunks(ctx, result); err != nil {
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

	if err := b.uploadChunks(ctx, result); err != nil {
		return err
	}

	return nil
}

// uploadChunks attempts to upload multiple chunks to the configured client.
// In case of failure all the events are added back to the buffer.
func (b *eventBuffer) uploadChunks(ctx context.Context, result [][]byte) error {
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

// readEvent does a nonblocking read from the event buffer
func (b *eventBuffer) readEvent() *bufferItem {
	select {
	case event := <-b.buffer:
		return event
	default:
		return nil
	}
}
