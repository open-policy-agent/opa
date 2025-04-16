// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"

	"github.com/open-policy-agent/opa/v1/metrics"
)

const (
	encHardLimitThreshold              = 0.9
	softLimitBaseFactor                = 2
	softLimitExponentScaleFactor       = 0.2
	encLogExUploadSizeLimitCounterName = "enc_log_exceeded_upload_size_limit_bytes"
	encSoftLimitScaleUpCounterName     = "enc_soft_limit_scale_up"
	encSoftLimitScaleDownCounterName   = "enc_soft_limit_scale_down"
	encSoftLimitStableCounterName      = "enc_soft_limit_stable"
)

// chunkEncoder implements log buffer chunking and compression.
// Decision events are written to the encoder and the encoder outputs chunks that are fit to the configured limit.
type chunkEncoder struct {
	// limit is the maximum compressed payload size (configured by upload_size_limit_bytes)
	limit int64
	// bytesWritten is used to track if anything has been written to the buffer
	// using this avoids working around the fact that the gzip compression adds a header
	bytesWritten int
	buf          *bytes.Buffer
	w            *gzip.Writer
	metrics      metrics.Metrics

	// The soft limit is a dynamic limit that will maximize the amount of events that fit in each chunk.
	// After creating a chunk it will determine if it should scale up and down based on the chunk size vs the limit.
	// If the chunk didn't reach the limit perhaps future events could have been added if the limit had been more flexible.
	// Same as the `limit` the soft limit enforces the final compressed size of the chunk
	softLimit                  int64
	softLimitScaleUpExponent   float64
	softLimitScaleDownExponent float64
}

func newChunkEncoder(limit int64) *chunkEncoder {
	enc := &chunkEncoder{
		limit:                      limit,
		softLimit:                  limit,
		softLimitScaleUpExponent:   0,
		softLimitScaleDownExponent: 0,
	}
	enc.update()

	return enc
}

func (enc *chunkEncoder) WithMetrics(m metrics.Metrics) *chunkEncoder {
	enc.metrics = m
	return enc
}

func (enc *chunkEncoder) Write(event EventV1) (result [][]byte, err error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(event); err != nil {
		return nil, err
	}

	return enc.WriteBytes(buf.Bytes())
}

// WriteBytes attempts to write a serialized event to the current chunk.
// If the upload limit is reached the chunk is closed and a result is returned.
// The incoming event that didn't fit is added to the next chunk.
func (enc *chunkEncoder) WriteBytes(serializedEvent []byte) ([][]byte, error) {
	if len(serializedEvent) == 0 {
		return nil, nil
	}

	// Compress the incoming event by itself in order to verify its final size
	// this buffer and gzip writer are only used temporarily to get the correct lengths
	buf := new(bytes.Buffer)
	w := gzip.NewWriter(buf)
	if _, err := w.Write(serializedEvent); err != nil {
		return nil, err
	}
	if _, err := w.Write([]byte(`]`)); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	// lenLastEvent is the length of the event if it was the last event added, including gzip(event+]+trailer)
	// a gzip header is 10 bytes, a trailer 8 bytes therefore -2 to keep an extra 8 bytes without having to write the trailer
	lenLastEvent := int64(buf.Len()) - 2

	if _, err := w.Write([]byte(`[`)); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	// lenEventChunk is the length of the incoming event within a single chunk, including gzip(header+[+event+]+trailer)
	lenEventChunk := int64(buf.Len())
	// if the compressed incoming event is too big to even fit in a single chunk by itself then reject it
	if lenEventChunk > enc.limit {
		if enc.metrics != nil {
			enc.metrics.Counter(encLogExUploadSizeLimitCounterName).Incr()
		}
		return nil, fmt.Errorf("received a decision event with size %d that exceeds the upload_size_limit_bytes %d",
			lenEventChunk, enc.limit)
	}

	if err := enc.w.Flush(); err != nil {
		return nil, err
	}

	// If gzip(header+[+existingEvents+incomingEvent+]+trailer) exceed the limit,
	// close the current chunk and reset it so the incoming event can be written into the next chunk.
	// Note that the soft limit enforces the final compressed size.
	var result [][]byte
	if lenLastEvent+int64(enc.buf.Len()) > enc.softLimit {
		if err := enc.writeClose(); err != nil {
			return nil, err
		}

		var err error
		result, err = enc.reset()
		if err != nil {
			return nil, err
		}
	}

	if enc.bytesWritten == 0 {
		n, err := enc.w.Write([]byte(`[`))
		if err != nil {
			return nil, err
		}
		enc.bytesWritten += n
	} else {
		n, err := enc.w.Write([]byte(`,`))
		if err != nil {
			return nil, err
		}
		enc.bytesWritten += n
	}

	// write the original serializedEvent to the final chunk
	n, err := enc.w.Write(serializedEvent)
	if err != nil {
		return nil, err
	}
	enc.bytesWritten += n

	if result != nil {
		return result, nil
	}

	return nil, nil
}

func (enc *chunkEncoder) writeClose() error {
	if _, err := enc.w.Write([]byte(`]`)); err != nil {
		return err
	}
	return enc.w.Close()
}

func (enc *chunkEncoder) Flush() ([][]byte, error) {
	if enc.bytesWritten == 0 {
		return nil, nil
	}
	if err := enc.writeClose(); err != nil {
		return nil, err
	}
	return enc.reset()
}

func (enc *chunkEncoder) reset() ([][]byte, error) {

	// Adjust the encoder's soft limit based on the current amount of
	// data written to the underlying buffer. The soft limit decides when to flush a chunk.
	// The soft limit is modified based on the below algorithm:
	// 1) Scale Up: If the current chunk size is within 90% of the user-configured limit, exponentially increase
	// the soft limit. The exponential function is 2^x where x has a minimum value of 1
	// 2) Scale Down: If the current chunk size exceeds the hard limit, decrease the soft limit and re-encode the
	// decisions in the last chunk.
	// 3) Equilibrium: If the chunk size is between 90% and 100% of the user-configured limit, maintain soft limit value.

	// 1) Scale Up
	if enc.buf.Len() < int(float64(enc.limit)*encHardLimitThreshold) {
		if enc.metrics != nil {
			enc.metrics.Counter(encSoftLimitScaleUpCounterName).Incr()
		}

		mul := int64(math.Pow(float64(softLimitBaseFactor), float64(enc.softLimitScaleUpExponent+1)))
		enc.softLimit *= mul
		enc.softLimitScaleUpExponent += softLimitExponentScaleFactor
		return enc.update(), nil
	}

	// 3) Equilibrium
	if int(enc.limit) > enc.buf.Len() && enc.buf.Len() >= int(float64(enc.limit)*encHardLimitThreshold) {
		if enc.metrics != nil {
			enc.metrics.Counter(encSoftLimitStableCounterName).Incr()
		}

		enc.softLimitScaleDownExponent = enc.softLimitScaleUpExponent
		return enc.update(), nil
	}

	// 2) Scale Down
	if enc.softLimit > enc.limit {
		if enc.metrics != nil {
			enc.metrics.Counter(encSoftLimitScaleDownCounterName).Incr()
		}

		if enc.softLimitScaleDownExponent < enc.softLimitScaleUpExponent {
			enc.softLimitScaleDownExponent = enc.softLimitScaleUpExponent
		}

		den := int64(math.Pow(float64(softLimitBaseFactor), float64(enc.softLimitScaleDownExponent-enc.softLimitScaleUpExponent+1)))
		enc.softLimit /= den

		if enc.softLimitScaleUpExponent > 0 {
			enc.softLimitScaleUpExponent -= softLimitExponentScaleFactor
		}
	}

	events, decErr := newChunkDecoder(enc.buf.Bytes()).decode()
	if decErr != nil {
		return nil, decErr
	}

	enc.initialize()

	var result [][]byte
	for i := range events {
		chunk, err := enc.Write(events[i])
		if err != nil {
			return nil, err
		}

		if chunk != nil {
			result = append(result, chunk...)
		}
	}
	return result, nil
}

func (enc *chunkEncoder) update() [][]byte {
	buf := enc.buf
	enc.initialize()
	if buf != nil {
		return [][]byte{buf.Bytes()}
	}
	return nil
}

func (enc *chunkEncoder) initialize() {
	enc.buf = new(bytes.Buffer)
	enc.bytesWritten = 0
	enc.w = gzip.NewWriter(enc.buf)
}

// chunkDecoder decodes the encoded chunks and outputs the log events
type chunkDecoder struct {
	raw []byte
}

func newChunkDecoder(raw []byte) *chunkDecoder {
	return &chunkDecoder{
		raw: raw,
	}
}

func (dec *chunkDecoder) decode() ([]EventV1, error) {
	gr, err := gzip.NewReader(bytes.NewReader(dec.raw))
	if err != nil {
		return nil, err
	}

	var events []EventV1
	if err := json.NewDecoder(gr).Decode(&events); err != nil {
		return nil, err
	}

	return events, gr.Close()
}
