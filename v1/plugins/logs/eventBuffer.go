package logs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/v1/plugins/rest"
)

// eventBuffer stores and uploads a gzip compressed JSON array of EventV1 entries
type eventBuffer struct {
	buffer               chan []byte    // buffer stores JSON encoded EventV1 data
	mtx                  sync.RWMutex   // mtx controls access to the buffer
	payload              *payloadBuffer // payload contains the compressed JSON array to be uploaded
	client               rest.Client    // client is used to upload the data to the configured service
	uploadPath           string         // uploadPath is the configured HTTP resource path for upload
	uploadSizeLimitBytes int64          // uploadSizeLimitBytes will enforce a maximum payload size to be uploaded
}

func newEventBuffer(bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) *eventBuffer {
	return &eventBuffer{
		buffer:               make(chan []byte, bufferSizeLimitEvents),
		payload:              newPayloadBuffer(),
		client:               client,
		uploadPath:           uploadPath,
		uploadSizeLimitBytes: uploadSizeLimitBytes,
	}
}

// Reconfigure updates the user configurable values
func (b *eventBuffer) Reconfigure(bufferSizeLimitEvents int64, client rest.Client, uploadPath string, uploadSizeLimitBytes int64) (int, []error) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.client = client
	b.uploadPath = uploadPath
	oldUploadSizeLimitBytes := b.uploadSizeLimitBytes
	b.uploadSizeLimitBytes = uploadSizeLimitBytes

	if int64(cap(b.buffer)) == bufferSizeLimitEvents && uploadSizeLimitBytes == oldUploadSizeLimitBytes {
		return 0, nil
	}

	close(b.buffer)
	newBuffer := make(chan []byte, bufferSizeLimitEvents)

	var errs []error
	var dropped int
	for event := range b.buffer {
		if int64(len(event)) < b.uploadSizeLimitBytes {
			if push(newBuffer, event) {
				dropped++
			}
			continue
		}

		var decoded EventV1
		if err := json.Unmarshal(event, &decoded); err != nil {
			dropped++
			errs = append(errs, err)
			continue
		}
		if err := b.dropNDCache(&decoded, event); err != nil {
			if !errors.Is(err, droppedNDCache{}) {
				dropped++
				errs = append(errs, err)
				continue
			}
			event, err = json.Marshal(decoded)
			if err != nil {
				dropped++
				errs = append(errs, err)
				continue
			}
		}
		if push(newBuffer, event) {
			dropped++
		}
	}
	b.buffer = newBuffer

	return dropped, errs
}

// Push attempts to add a new event to the buffer, returning true if an event was dropped.
func (b *eventBuffer) Push(event EventV1) (bool, error) {
	encoded, err := json.Marshal(event)
	if err != nil {
		return false, err
	}

	err = b.dropNDCache(&event, encoded)
	if err != nil {
		if !errors.As(err, &droppedNDCache{}) {
			return false, err
		}
	}

	// This only blocks when the buffer is being reconfigured
	b.mtx.RLock()
	defer b.mtx.RUnlock()

	return push(b.buffer, encoded), err
}

// dropNDCache checks if an event is bigger than UploadSizeLimitBytes and
// tries to drop the NDBuiltinCache. If the event is still too big, drop the event.
func (b *eventBuffer) dropNDCache(event *EventV1, encoded []byte) error {
	if int64(len(encoded)) < b.uploadSizeLimitBytes {
		return nil
	}

	if event.NDBuiltinCache == nil {
		return fmt.Errorf("upload chunk size (%d) exceeds upload_size_limit_bytes (%d)",
			int64(len(encoded)), b.uploadSizeLimitBytes)
	}
	event.NDBuiltinCache = nil
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if int64(len(encoded)) > b.uploadSizeLimitBytes {
		return fmt.Errorf("upload chunk size (%d) exceeds upload_size_limit_bytes (%d)",
			int64(len(encoded)), b.uploadSizeLimitBytes)
	}

	return droppedNDCache{}
}

// push adds data to a channel, if the channel is full the oldest data is dropped (FIFO).
func push[T any](ch chan T, data T) bool {
	select {
	case ch <- data:
		return false
	default:
		<-ch
		ch <- data
		return true
	}
}

// Upload reads all the currently buffered events and uploads them as a gzip compressed JSON array to the service.
// The events are uploaded in chunks limited by the uploadSizeLimitBytes.
func (b *eventBuffer) Upload(ctx context.Context) (bool, error) {
	b.payload.mtx.Lock()
	defer b.payload.mtx.Unlock()

	// This only blocks when the buffer is being reconfigured
	b.mtx.RLock()
	defer b.mtx.RUnlock()

	events := len(b.buffer)
	for range events {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case event := <-b.buffer:
			if !b.payload.capacity(len(event), b.uploadSizeLimitBytes) {
				ok, err := b.payload.upload(ctx, b.client, b.uploadPath)
				if !ok || err != nil {
					return false, err
				}
			}

			if err := b.payload.add(event); err != nil {
				return false, err
			}
		default:
			break
		}
	}

	return b.payload.upload(ctx, b.client, b.uploadPath)
}

// payloadBuffer contains the compressed JSON array to be uploaded
type payloadBuffer struct {
	buffer       *bytes.Buffer
	mtx          sync.Mutex
	writer       *gzip.Writer
	closed       bool
	bytesWritten int
}

func newPayloadBuffer() *payloadBuffer {
	p := payloadBuffer{}
	p.reset()
	return &p
}

func (p *payloadBuffer) capacity(eventLen int, uploadSizeLimitBytes int64) bool {
	return !(int64(eventLen+p.bytesWritten+1) > uploadSizeLimitBytes)
}

// add writes an event to the JSON array
func (p *payloadBuffer) add(event []byte) error {
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

	n, err := p.writer.Write(event)
	if err != nil {
		return err
	}

	p.bytesWritten += n

	return nil
}

// upload closes the JSON array and attempts to send the data to the configured service
func (p *payloadBuffer) upload(ctx context.Context, client rest.Client, uploadPath string) (bool, error) {
	if p.bytesWritten == 0 {
		return false, nil
	}

	if !p.closed {
		_, err := p.writer.Write([]byte(`]`))
		if err != nil {
			return false, err
		}

		p.closed = true
		if err := p.writer.Close(); err != nil {
			return false, err
		}
	}

	if err := uploadChunk(ctx, client, uploadPath, p.buffer.Bytes()); err != nil {
		return false, err
	}

	p.reset()

	return true, nil
}

func (p *payloadBuffer) reset() {
	p.bytesWritten = 0
	p.buffer = new(bytes.Buffer)
	p.writer = gzip.NewWriter(p.buffer)
	p.closed = false
}
