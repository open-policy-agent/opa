package logs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"sync"

	"github.com/open-policy-agent/opa/v1/plugins/rest"
)

// bufferItem contains is an extended EventV1 to also hold the serialized version to avoid re-serialization.
type bufferItem struct {
	EventV1
	serialized []byte
}

// eventBuffer stores and uploads a gzip compressed JSON array of EventV1 entries
type eventBuffer struct {
	buffer               chan bufferItem // buffer stores JSON encoded EventV1 data
	payload              *payload        // payload contains the compressed JSON array to be uploaded
	upload               sync.Mutex      // upload controls that uploads are done sequentially
	client               rest.Client     // client is used to upload the data to the configured service
	uploadPath           string          // uploadPath is the configured HTTP resource path for upload
	uploadSizeLimitBytes int64           // uploadSizeLimitBytes will enforce a maximum payload size to be uploaded
}

func newEventBuffer(bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) *eventBuffer {
	return &eventBuffer{
		buffer:               make(chan bufferItem, bufferSizeLimitEvents),
		payload:              newPayload(),
		client:               client,
		uploadPath:           uploadPath,
		uploadSizeLimitBytes: uploadSizeLimitBytes,
	}
}

// Reconfigure updates the user configurable values
func (b *eventBuffer) Reconfigure(p *Plugin, bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) {
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
		b.Push(p, event)
	}
}

// Push attempts to add a new event to the buffer, returning true if an event was dropped.
func (b *eventBuffer) Push(p *Plugin, event bufferItem) {
	select {
	case b.buffer <- event:
	default:
		<-b.buffer
		b.buffer <- event
		p.incrMetric(logBufferEventLimitExDropCounterName)
	}
}

// Upload reads all the currently buffered events and uploads them as a gzip compressed JSON array to the service.
// The events are uploaded as chunks limited by the uploadSizeLimitBytes.
func (b *eventBuffer) Upload(ctx context.Context, p *Plugin) (bool, error) {
	b.upload.Lock()
	defer b.upload.Unlock()

	b.compress(p)
	return b.payload.upload(ctx, b.client, b.uploadPath)
}

// compress will read events from the buffer creating a gzipped compressed JSON array for upload.
// events are dropped if it exceeds the upload size limit or there is a marshalling problem.
func (b *eventBuffer) compress(p *Plugin) {
	// previous payload hasn't been sent and needs to be retried
	if b.payload.bytesWritten != 0 {
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
					p.incrMetric(logEncodingFailureCounterName)
					p.logger.Error("encoding failure: %v, dropping event with decision ID: %v.", err, event.EventV1.DecisionID)
					continue
				}
			}

			if int64(len(event.serialized)) >= b.uploadSizeLimitBytes {
				if event.NDBuiltinCache == nil {
					p.logger.Error("upload event size (%d) exceeds upload_size_limit_bytes (%d), dropping event with decision ID: %v.",
						int64(len(event.serialized)), b.uploadSizeLimitBytes, event.EventV1.DecisionID)
					continue
				}

				// Attempt to drop the ND cache to reduce size. If it is still too large, drop the event.
				event.NDBuiltinCache = nil
				var err error
				event.serialized, err = json.Marshal(event.EventV1)
				if err != nil {
					p.incrMetric(logEncodingFailureCounterName)
					p.logger.Error("encoding failure: %v, dropping event with decision ID: %v.", err, event.EventV1.DecisionID)
					continue
				}
				if int64(len(event.serialized)) > b.uploadSizeLimitBytes {
					p.logger.Error("upload event size (%d) exceeds upload_size_limit_bytes (%d), dropping event with decision ID: %v.",
						int64(len(event.serialized)), b.uploadSizeLimitBytes, event.EventV1.DecisionID)
					continue
				}

				p.incrMetric(logNDBDropCounterName)
				p.logger.Error("ND builtins cache dropped from this event to fit under maximum upload size limits. Increase upload size limit or change usage of non-deterministic builtins.")
			}

			if !b.payload.capacity(len(event.serialized), b.uploadSizeLimitBytes) {
				// add the event that doesn't fit back into the buffer
				b.Push(p, event)
				break
			}

			if err := b.payload.add(event); err != nil {
				// corrupted payload, record failure and reset
				b.payload.reset()
				p.incrMetric(logEncodingFailureCounterName)
				p.logger.Error("encoding failure: %v, dropping event with decision ID: %v.", err, event.EventV1.DecisionID)
				continue
			}
		default:
			break
		}
	}

	if err := b.payload.close(); err != nil {
		// corrupted payload, record failure and reset
		b.payload.reset()
		p.incrMetric(logEncodingFailureCounterName)
		p.logger.Error("encoding failure: %v, dropping multiple events.")
		return
	}
}

// payload contains the compressed JSON array to be uploaded
type payload struct {
	buffer       *bytes.Buffer
	writer       *gzip.Writer
	bytesWritten int
}

func newPayload() *payload {
	p := payload{}
	p.reset()
	return &p
}

func (p *payload) capacity(eventLen int, uploadSizeLimitBytes int64) bool {
	return !(int64(eventLen+p.bytesWritten+1) > uploadSizeLimitBytes)
}

// add writes an event to the JSON array
func (p *payload) add(event bufferItem) error {
	switch p.bytesWritten {
	case 0: // Start new JSON array
		n, err := p.writer.Write([]byte(`[`))
		if err != nil {
			return err
		}
		p.bytesWritten += n
	default: // Append new event to JSON array
		n, err := p.writer.Write([]byte(`,`))
		if err != nil {
			return err
		}
		p.bytesWritten += n
	}

	n, err := p.writer.Write(event.serialized)
	if err != nil {
		return err
	}

	p.bytesWritten += n

	return nil
}

// close writes the closing bracket to the array
func (p *payload) close() error {
	n, err := p.writer.Write([]byte(`]`))
	if err != nil {
		return err
	}
	p.bytesWritten += n

	if err := p.writer.Close(); err != nil {
		return err
	}

	return nil
}

// upload closes the JSON array and attempts to send the data to the configured service
func (p *payload) upload(ctx context.Context, client rest.Client, uploadPath string) (bool, error) {
	if p.bytesWritten == 0 {
		return false, nil
	}

	if err := uploadChunk(ctx, client, uploadPath, p.buffer.Bytes()); err != nil {
		return false, err
	}

	p.reset()

	return true, nil
}

func (p *payload) reset() {
	p.bytesWritten = 0
	p.buffer = new(bytes.Buffer)
	p.writer = gzip.NewWriter(p.buffer)
}
