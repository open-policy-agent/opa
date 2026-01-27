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
	"github.com/open-policy-agent/opa/v1/plugins"
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
	buffer     chan *bufferItem // buffer stores JSON encoded EventV1 data
	uploadLock sync.Mutex
	enc        *chunkEncoder // enc adds events into a gzip compressed JSON array (chunk)
	limiter    *rate.Limiter
	metrics    metrics.Metrics
	logger     logging.Logger
	client     rest.Client
	uploadPath string
	// Enables the read loop in immediate mode to constantly read from the event buffer
	mode         plugins.TriggerMode
	stop         chan chan struct{}
	cancelUpload bool
}

func newEventBuffer(bufferSizeLimitEvents int64, uploadSizeLimitBytes int64, client rest.Client, uploadPath string, mode plugins.TriggerMode) *eventBuffer {
	b := &eventBuffer{
		buffer:     make(chan *bufferItem, bufferSizeLimitEvents),
		enc:        newChunkEncoder(uploadSizeLimitBytes),
		mode:       mode,
		client:     client,
		uploadPath: uploadPath,
	}

	if b.mode == plugins.TriggerImmediate {
		b.stop = make(chan chan struct{})
		go b.read()
	}

	return b
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

func (b *eventBuffer) Flush() []*EventV1 {
	var events []*EventV1

	result, err := b.enc.Flush()
	if err != nil {
		b.incrMetric(logEncodingFailureCounterName)
		if b.logger != nil {
			b.logger.Error("Failed to upload decision logs, events have been buffered an will be retried.")
		}
		return nil
	}

	for _, r := range result {
		decodedEvents, err := newChunkDecoder(r).decode()
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

	lenEvents := len(b.buffer)
	if lenEvents == 0 {
		return events
	}

	for range lenEvents {
		event := <-b.buffer
		if event.EventV1 != nil {
			events = append(events, event.EventV1)
		} else if event.chunk != nil {
			decodedEvents, err := newChunkDecoder(event.chunk).decode()
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
	}

	return events
}

func (b *eventBuffer) Stop(ctx context.Context) {
	if b.mode != plugins.TriggerImmediate {
		return
	}
	done := make(chan struct{})
	b.stop <- done

	select {
	case <-done:
	case <-ctx.Done():
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
		if b.logger != nil {
			b.logger.Error("Decision log dropped as rate limit exceeded. Reduce reporting interval or increase rate limit.")
		}
		return
	}

	util.PushFIFO(b.buffer, event, b.metrics, logBufferEventDropCounterName)
}

func (b *eventBuffer) processBufferItem(item *bufferItem) [][]byte {
	if item.chunk != nil {
		return [][]byte{item.chunk}
	}

	var result [][]byte
	event := item.EventV1
	eventBytes, err := json.Marshal(&event)
	if err != nil {
		b.incrMetric(logEncodingFailureCounterName)
		if b.logger != nil {
			b.logger.Error("Dropping event due to encoding failure with decision ID: %v", event.DecisionID)
		}
		return nil
	}

	result, err = b.enc.Encode(*event, eventBytes)
	if err != nil {
		b.incrMetric(logEncodingFailureCounterName)
		if b.logger != nil {
			b.logger.Error("Dropping event due to encoding failure with decision ID: %v", event.DecisionID)
		}
		return nil
	}

	return result
}

func (b *eventBuffer) immediateRead(ctx context.Context, item *bufferItem) {
	b.uploadLock.Lock()
	defer b.uploadLock.Unlock()

	result := b.processBufferItem(item)
	if result == nil {
		return
	}

	if err := b.uploadChunks(ctx, result, b.client, b.uploadPath); err != nil {
		if b.logger != nil {
			b.logger.Error("Failed to upload decision logs, events have been buffered an will be retried. Error: %v", err)
		}
	}

	b.cancelUpload = true
}

// read is a loop that reads from the buffer constantly, so that the chunk can be uploaded as soon as it is ready
func (b *eventBuffer) read() {
	ctx := context.Background()
	for {
		select {
		case item := <-b.buffer:
			b.immediateRead(ctx, item)
		case done := <-b.stop:
			b.uploadLock.Lock()
			// reset so that Upload can be used to attempt final upload before shutting down
			b.cancelUpload = false
			done <- struct{}{}
			b.uploadLock.Unlock()
			return
		}
	}
}

// Upload reads events from the buffer and uploads them to the configured client.
// All the events currently in the buffer are read and written to a gzip compressed JSON array to create a chunk of data.
// Each chunk is limited by the uploadSizeLimitBytes.
func (b *eventBuffer) Upload(ctx context.Context) error {
	b.uploadLock.Lock()
	defer b.uploadLock.Unlock()

	// prevent uploading after already uploading from immediate upload loop
	if b.cancelUpload {
		return nil
	}

	eventLen := len(b.buffer)

	for range eventLen {
		item := b.readBufItem()
		if item == nil {
			break
		}

		result := b.processBufferItem(item)
		if result != nil {
			if err := b.uploadChunks(ctx, result, b.client, b.uploadPath); err != nil {
				if b.logger != nil {
					b.logger.Error("Failed to upload decision logs, events have been buffered an will be retried. Error: %v", err)
				}
			}
		}
	}

	// flush any chunks that didn't hit the upload limit
	result, err := b.enc.Flush()
	if err != nil {
		b.incrMetric(logEncodingFailureCounterName)
		if b.logger != nil {
			b.logger.Error("Failed to upload decision logs, events have been buffered an will be retried.")
		}
		return nil
	}

	if result == nil {
		return nil
	}

	if err := b.uploadChunks(ctx, result, b.client, b.uploadPath); err != nil {
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
