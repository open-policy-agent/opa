// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"math"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
)

const (
	encCompressedLimitThreshold              = 0.9
	uncompressedLimitBaseFactor              = 2
	uncompressedLimitExponentScaleFactor     = 0.2
	logNDBDropCounterName                    = "decision_logs_nd_builtin_cache_dropped"
	encLogExUploadSizeLimitCounterName       = "enc_log_exceeded_upload_size_limit_bytes"
	encUncompressedLimitScaleUpCounterName   = "enc_uncompressed_limit_scale_up"
	encUncompressedLimitScaleDownCounterName = "enc_uncompressed_limit_scale_down"
	encUncompressedLimitStableCounterName    = "enc_uncompressed_limit_stable"
	encSoftLimitScaleUpCounterName           = "enc_soft_limit_scale_up"   // deprecated, use uncompressed version instead
	encSoftLimitScaleDownCounterName         = "enc_soft_limit_scale_down" // deprecated, use uncompressed version instead
	encSoftLimitStableCounterName            = "enc_soft_limit_stable"     // deprecated, use uncompressed version instead
	encNumberOfEventsInChunkHistogramName    = "enc_events_written_in_chunk"
)

// chunkEncoder implements log buffer chunking and compression.
// Decision events are written to the encoder and the encoder outputs chunks that are fit to the configured limit.
type chunkEncoder struct {
	// limit is the maximum compressed payload size (configured by upload_size_limit_bytes)
	limit     int64
	threshold int
	// bytesWritten is used to track if anything has been written to the buffer
	// using this avoids working around the fact that the gzip compression adds a header
	bytesWritten  int
	eventsWritten int64
	buf           *bytes.Buffer
	w             *gzip.Writer
	metrics       metrics.Metrics
	logger        logging.Logger
	// lastDroppedNDSize is a known size of an individual event that would require the ND cache to be dropped
	lastDroppedNDSize int64

	// The uncompressedLimit is an adaptive limit that will attempt to guess the uncompressedLimit based on the utilization of the buffer on upload.
	// This minimizes having to decompress all the events in case the limit is reached, needing to only do it if the guess is too large.
	// Otherwise, you would need to compress the incoming event by itself to get an accurate size for comparison which would cause two compressions each write.
	// This means that at first the chunks will contain fewer events until the uncompressedLimit can grow to a stable state.
	uncompressedLimit                  int64
	uncompressedLimitScaleUpExponent   float64
	uncompressedLimitScaleDownExponent float64
}

func newChunkEncoder(limit int64) *chunkEncoder {
	enc := &chunkEncoder{
		limit:                              limit,
		uncompressedLimit:                  limit,
		threshold:                          int(float64(limit) * encCompressedLimitThreshold),
		uncompressedLimitScaleUpExponent:   0,
		uncompressedLimitScaleDownExponent: 0,
	}
	enc.initialize()

	return enc
}

func (enc *chunkEncoder) Reconfigure(limit int64) {
	enc.limit = limit
	enc.uncompressedLimit = limit
	enc.uncompressedLimitScaleUpExponent = 0
	enc.uncompressedLimitScaleDownExponent = 0
	enc.threshold = int(float64(limit) * encCompressedLimitThreshold)
	enc.lastDroppedNDSize = 0
}

// WithUncompressedLimit keep the adaptive uncompressed limit throughout the lifecycle of the size buffer
// this ensures that the uncompressed limit can grow/shrink appropriately as new data comes in
func (enc *chunkEncoder) WithUncompressedLimit(uncompressedLimit int64, uncompressedLimitScaleDownExponent float64, uncompressedLimitScaleUpExponent float64) *chunkEncoder {
	enc.uncompressedLimit = uncompressedLimit
	enc.uncompressedLimitScaleUpExponent = uncompressedLimitScaleUpExponent
	enc.uncompressedLimitScaleDownExponent = uncompressedLimitScaleDownExponent
	return enc
}

func (enc *chunkEncoder) WithMetrics(m metrics.Metrics) *chunkEncoder {
	enc.metrics = m
	return enc
}

func (enc *chunkEncoder) WithLogger(logger logging.Logger) *chunkEncoder {
	enc.logger = logger
	return enc
}

func (enc *chunkEncoder) scaleUp() {
	enc.incrMetric(encUncompressedLimitScaleUpCounterName)
	enc.incrMetric(encSoftLimitScaleUpCounterName)

	mul := int64(math.Pow(float64(uncompressedLimitBaseFactor), enc.uncompressedLimitScaleUpExponent+1))
	enc.uncompressedLimit *= mul
	enc.uncompressedLimitScaleUpExponent += uncompressedLimitExponentScaleFactor
}

// Encode attempts to write an encoded event to the current chunk.
// A chunk is returned when it reaches the uncompressed limit, the uncompressed limit is adjusted if the buffer was underutilized or exceeded.
// An event is only dropped if it exceeds the limit after being compressed with or without dropping the Non-deterministic Cache (NDBuiltinCache).
// An event stays in the buffer until either a new event reaches the uncompressed limit or by calling Flush.
func (enc *chunkEncoder) Encode(event EventV1, eventBytes []byte) ([][]byte, error) {
	// the incoming event is too big without dropping the ND cache
	if enc.lastDroppedNDSize != 0 && int64(len(eventBytes)) >= enc.lastDroppedNDSize {
		if event.NDBuiltinCache == nil {
			enc.incrMetric(logEncodingFailureCounterName)
			if enc.logger != nil {
				enc.logger.Error("Log encoding failed: received a decision event size (%d) that exceeded the upload_size_limit_bytes (%d). No ND cache to drop.",
					len(eventBytes), enc.limit)
			}
			return nil, nil
		}

		// re-encode the event with the ND cache removed
		event.NDBuiltinCache = nil
		if enc.logger != nil {
			enc.logger.Error("ND builtins cache dropped from this event to fit under maximum upload size limits. Increase upload size limit or change usage of non-deterministic builtins.")
		}
		enc.incrMetric(logNDBDropCounterName)

		var err error
		eventBytes, err = json.Marshal(&event)
		if err != nil {
			return nil, err
		}
	}

	if int64(len(eventBytes)+enc.bytesWritten+1) <= enc.uncompressedLimit {
		return nil, enc.appendEvent(eventBytes)
	}

	// Adjust the encoder's uncompressed limit based on the current amount of
	// data written to the underlying buffer. The uncompressed limit decides when to return a chunk.
	// The uncompressed limit is modified based on the below algorithm:
	// 1) Scale Up: If the current chunk size is below 90% of the user-configured limit, exponentially increase
	// the uncompressed limit. The exponential function is 2^x where x has a minimum value of 1.
	// A chunk will be returned with what was already written, the buffer was underutilized but the next time it shouldn't be.
	// The incoming event is written to the next chunk.
	// 2) Scale Down: If the current chunk size exceeds the compressed limit, decrease the uncompressed limit and re-encode the
	// decisions in the last chunk.
	// 3) Equilibrium: If the chunk size is between 90% and 100% of the user-configured limit, maintain uncompressed limit value.
	// A chunk will be returned with what was already written, the uncompressed limit is ideal.
	// The incoming event is written to the next chunk.

	// The uncompressed size is too small (it starts equal to the compressed limit)
	// Or this is a recursive call to Write trying to split the events into separate chunks
	if enc.bytesWritten == 0 {
		// If an event is too large, there are multiple things to try before dropping the event:
		// 1. Try to fit the incoming event into the next chunk without losing ND cache
		if err := enc.appendEvent(eventBytes); err != nil {
			return nil, err
		}

		result, err := enc.reset()
		if err != nil {
			return nil, err
		}

		currentSize := len(result)
		if currentSize < int(enc.limit) {
			// success! the incoming chunk doesn't have to lose the ND cache and can go into a chunk by itself
			// scale up the uncompressed limit using the uncompressed event size as a base
			err = enc.appendEvent(eventBytes)
			if err != nil {
				return nil, err
			}
			enc.uncompressedLimit = int64(len(eventBytes))
			enc.scaleUp()
			return nil, nil
		}

		// The ND cache has to be dropped, record this size as a known maximum event size
		if enc.lastDroppedNDSize == 0 || int64(len(eventBytes)) < enc.lastDroppedNDSize {
			enc.lastDroppedNDSize = int64(len(eventBytes))
		}

		// 2. Drop the ND cache and see if the incoming event can fit within the current chunk without the cache (so we can maximize chunk size)
		enc.initialize()
		enc.incrMetric(encLogExUploadSizeLimitCounterName)
		// If there's no ND builtins cache in the event, then we don't need to retry encoding anything.
		if event.NDBuiltinCache == nil {
			enc.incrMetric(logEncodingFailureCounterName)
			if enc.logger != nil {
				enc.logger.Error("Log encoding failed: received a decision event size (%d) that exceeded the upload_size_limit_bytes (%d). No ND cache to drop.", currentSize, enc.limit)
			}
			return nil, nil
		}
		// re-encode the event with the ND cache removed
		event.NDBuiltinCache = nil

		eventBytes, err = json.Marshal(&event)
		if err != nil {
			return nil, err
		}
		err = enc.appendEvent(eventBytes)
		if err != nil {
			return nil, err
		}

		result, err = enc.reset()
		if err != nil {
			return nil, err
		}

		if len(result) > int(enc.limit) {
			enc.incrMetric(logEncodingFailureCounterName)
			if enc.logger != nil {
				enc.logger.Error("Log encoding failed: received a decision event size (%d) that exceeded the upload_size_limit_bytes (%d) even after dropping the ND cache.",
					len(eventBytes), enc.limit)
			}
			enc.initialize() // drop the event
			return nil, nil
		}

		// success! the incoming event without its ND cache fits into the current chunk
		err = enc.appendEvent(eventBytes)
		if err != nil {
			return nil, err
		}
		if enc.logger != nil {
			enc.logger.Error("ND builtins cache dropped from this event to fit under maximum upload size limits. Increase upload size limit or change usage of non-deterministic builtins.")
		}
		enc.incrMetric(logNDBDropCounterName)
		// success! the incoming chunk lost the ND cache, but it wasn't dropped entirely
		// scale up the uncompressed limit using the uncompressed event size as a base
		if int64(len(eventBytes)) > enc.uncompressedLimit {
			enc.uncompressedLimit = int64(len(eventBytes))
		}
		enc.scaleUp()
		return nil, nil
	}

	enc.updateMetric(encNumberOfEventsInChunkHistogramName, enc.eventsWritten)
	result, err := enc.reset()
	if err != nil {
		return nil, err
	}

	// 1) Scale Up: If the current chunk size is below 90% of the user-configured limit, exponentially increase
	// the uncompressed limit. The exponential function is 2^x where x has a minimum value of 1
	if len(result) < enc.threshold {
		enc.scaleUp()

		results := [][]byte{result}

		r, err := enc.Encode(event, eventBytes)
		if err != nil {
			return results, err
		}

		if r != nil {
			results = append(results, r...)
		}

		return results, nil
	}

	// 3) Equilibrium: If the chunk size is between 90% and 100% of the user-configured limit, maintain uncompressed limit value.
	if int(enc.limit) > len(result) && len(result) >= enc.threshold {
		enc.incrMetric(encUncompressedLimitStableCounterName)
		enc.incrMetric(encSoftLimitStableCounterName)

		enc.uncompressedLimitScaleDownExponent = enc.uncompressedLimitScaleUpExponent

		results := [][]byte{result}

		r, err := enc.Encode(event, eventBytes)
		if err != nil {
			return results, err
		}

		if r != nil {
			results = append(results, r...)
		}

		return results, nil
	}

	// 2) Scale Down: If the current chunk size exceeds the compressed limit, decrease the uncompressed limit and re-encode the
	// decisions in the last chunk.
	events, err := newChunkDecoder(result).decode()
	if err != nil {
		return nil, err
	}

	// add the current event so that it can be reorganized as needed
	events = append(events, event)

	return enc.scaleDown(events)
}

func (enc *chunkEncoder) scaleDown(events []EventV1) ([][]byte, error) {
	if enc.uncompressedLimit > enc.limit {
		enc.incrMetric(encUncompressedLimitScaleDownCounterName)
		enc.incrMetric(encSoftLimitScaleDownCounterName)

		if enc.uncompressedLimitScaleDownExponent < enc.uncompressedLimitScaleUpExponent {
			enc.uncompressedLimitScaleDownExponent = enc.uncompressedLimitScaleUpExponent
		}

		den := int64(math.Pow(float64(uncompressedLimitBaseFactor), enc.uncompressedLimitScaleDownExponent-enc.uncompressedLimitScaleUpExponent+1))
		enc.uncompressedLimit /= den

		if enc.uncompressedLimitScaleUpExponent > 0 {
			enc.uncompressedLimitScaleUpExponent -= uncompressedLimitExponentScaleFactor
		}
	}

	// The uncompressed limit has grown too large the events need to be split up into multiple chunks
	enc.initialize()

	// split the events into multiple chunks
	var result [][]byte
	for i := range events {
		eventBytes, err := json.Marshal(&events[i])
		if err != nil {
			return nil, err
		}

		// recursive call to make sure the chunk created adheres to the uncompressed size limit
		chunks, err := enc.Encode(events[i], eventBytes)
		if err != nil {
			return nil, err
		}

		if chunks != nil {
			result = append(result, chunks...)
		}
	}

	return result, nil
}

func (enc *chunkEncoder) appendEvent(event []byte) error {
	if len(event) == 0 {
		return nil
	}

	if enc.bytesWritten == 0 {
		n, err := enc.w.Write([]byte(`[`))
		if err != nil {
			return err
		}
		enc.bytesWritten += n
	} else {
		n, err := enc.w.Write([]byte(`,`))
		if err != nil {
			return err
		}
		enc.bytesWritten += n
	}

	n, err := enc.w.Write(event)
	if err != nil {
		return err
	}
	enc.bytesWritten += n
	enc.eventsWritten++

	return nil
}

func (enc *chunkEncoder) writeClose() error {
	if _, err := enc.w.Write([]byte(`]`)); err != nil {
		return err
	}
	return enc.w.Close()
}

// Flush closes the current buffer and returns all the events written in chunks limited by the compressed limit.
// If the uncompressed size has grown too much it could require multiple decoding calls to size it down.
func (enc *chunkEncoder) Flush() ([][]byte, error) {
	if enc.bytesWritten == 0 {
		return nil, nil
	}

	defer enc.initialize()

	var result [][]byte

	// create chunks until the current buffer is smaller than the limit
	for {
		r, err := enc.reset()
		if err != nil {
			return nil, err
		}
		if len(r) < int(enc.limit) {
			return append(result, r), nil
		}
		events, err := newChunkDecoder(r).decode()
		if err != nil {
			return nil, err
		}
		chunk, err := enc.scaleDown(events)
		if err != nil {
			return nil, err
		}
		if chunk != nil {
			result = append(result, chunk...)
		}
	}
}

func (enc *chunkEncoder) reset() ([]byte, error) {
	if enc.bytesWritten == 0 {
		return nil, nil
	}

	defer enc.initialize()

	if err := enc.writeClose(); err != nil {
		return nil, err
	}

	return enc.buf.Bytes(), nil
}

func (enc *chunkEncoder) initialize() {
	enc.buf = new(bytes.Buffer)
	enc.bytesWritten = 0
	enc.eventsWritten = 0
	enc.w = gzip.NewWriter(enc.buf)
}

func (enc *chunkEncoder) incrMetric(name string) {
	if enc.metrics != nil {
		enc.metrics.Counter(name).Incr()
	}
}

func (enc *chunkEncoder) updateMetric(name string, value int64) {
	if enc.metrics != nil {
		enc.metrics.Histogram(name).Update(value)
	}
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
