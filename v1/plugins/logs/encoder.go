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
	encLogExUploadSizeLimitCounterName       = "enc_log_exceeded_upload_size_limit_bytes"
	encUncompressedLimitScaleUpCounterName   = "enc_uncompressed_limit_scale_up"
	encUncompressedLimitScaleDownCounterName = "enc_uncompressed_limit_scale_down"
	encUncompressedLimitStableCounterName    = "enc_uncompressed_limit_stable"
	encSoftLimitScaleUpCounterName           = "enc_soft_limit_scale_up"   // deprecated, use uncompressed version instead
	encSoftLimitScaleDownCounterName         = "enc_soft_limit_scale_down" // deprecated, use uncompressed version instead
	encSoftLimitStableCounterName            = "enc_soft_limit_stable"     // deprecated, use uncompressed version instead
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
	logger       logging.Logger

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
		uncompressedLimitScaleUpExponent:   0,
		uncompressedLimitScaleDownExponent: 0,
	}
	enc.update()

	return enc
}

func (enc *chunkEncoder) WithMetrics(m metrics.Metrics) *chunkEncoder {
	enc.metrics = m
	return enc
}

func (enc *chunkEncoder) Write(event EventV1) (result [][]byte, err error) {
	b, err := json.Marshal(&event)
	if err != nil {
		return nil, err
	}

	return enc.WriteBytes(b)
}

// WriteBytes attempts to write an encoded event to the current chunk.
// If the uncompressed limit is reached the chunk is closed and a result is returned.
// Then if the buffer was either underutilized or exceeded the compressed limit the uncompressed limit will be adjusted.
// The incoming event that didn't fit is added to the next chunk.
// The Non-deterministic Cache (NDBuiltinCache) can be dropped to attempt to fit the event into a chunk while adhering to the compressed limit.
func (enc *chunkEncoder) WriteBytes(incomingEventBytes []byte) ([][]byte, error) {
	if incomingEventBytes == nil {
		return nil, nil
	}

	var result [][]byte

	if enc.bytesWritten != 0 && int64(len(incomingEventBytes)+enc.bytesWritten+1) > enc.uncompressedLimit {
		if err := enc.writeClose(); err != nil {
			return nil, err
		}

		var err error
		result, err = enc.reset()
		if err != nil {
			return nil, err
		}
	}

	if err := enc.appendEvent(incomingEventBytes); err != nil {
		return nil, err
	}
	if err := enc.w.Flush(); err != nil {
		return nil, err
	}

	// adding the incomingEventBytes to the encoder caused it to exceed the limit!
	if enc.buf.Len() > int(enc.limit) {
		// reset the encoder to before the incomingEventBytes was written
		if err := enc.writeClose(); err != nil {
			return nil, err
		}
		events, decErr := newChunkDecoder(enc.buf.Bytes()).decode()
		if decErr != nil {
			return nil, decErr
		}

		enc.initialize()
		var incomingEvent EventV1
		for i := range events {
			// skip the incomingEventBytes but save it in case the ND cache needs to be dropped
			if i == len(events)-1 {
				incomingEvent = events[i]
				continue
			}
			// it was a valid chunk until the last event so no need to check anything
			b, err := json.Marshal(&events[i])
			if err != nil {
				return nil, err
			}
			err = enc.appendEvent(b)
			if err != nil {
				return nil, err
			}
		}

		// There are multiple things to try before dropping the event:
		// 1. Try to fit the incoming event into the next chunk without losing ND cache
		// 2. Try to drop the ND cache and see if the incoming event can fit with the current chunk without it (so we can maximize chunk size)
		// 3. Try to add the incoming event without its ND cache to a chunk by itself
		// 4. Drop the event, there isn't anything else to be done

		// 1. Try to fit the incoming event into the next chunk without losing ND cache
		tmpEncoder := newChunkEncoder(enc.limit)
		if err := tmpEncoder.appendEvent(incomingEventBytes); err != nil {
			return nil, err
		}
		if err := tmpEncoder.w.Flush(); err != nil {
			return nil, err
		}
		if tmpEncoder.buf.Len() < int(tmpEncoder.limit) {
			// success! the incoming chunk doesn't have to lose the ND cache and can go into the next chunk
			// flush current encoder buffer to make room for the next event
			if err := enc.writeClose(); err != nil {
				return nil, err
			}
			result = append(result, enc.update()...)

			// write event to empty chunk
			if err := enc.appendEvent(incomingEventBytes); err != nil {
				return nil, err
			}

			return result, nil
		}

		// 2. Try to drop the ND cache and see if the incoming event can fit with the current chunk without it (so we can maximize chunk size)
		enc.incrMetric(encLogExUploadSizeLimitCounterName)
		// If there's no ND builtins cache in the event, then we don't need to retry encoding anything.
		if incomingEvent.NDBuiltinCache == nil {
			enc.incrMetric(logEncodingFailureCounterName)
			enc.logError("Log encoding failed: received a decision event size (%d) that exceeded the upload_size_limit_bytes (%d). No ND cache to drop.",
				len(incomingEventBytes), enc.limit)
			return result, nil
		}
		// re-encode the event with the ND cache removed
		incomingEvent.NDBuiltinCache = nil
		incomingEventBytesDroppedNDCache, err := json.Marshal(&incomingEvent)
		if err != nil {
			return nil, err
		}
		err = enc.appendEvent(incomingEventBytesDroppedNDCache)
		if err != nil {
			return nil, err
		}
		if err := enc.w.Flush(); err != nil {
			return nil, err
		}
		if enc.buf.Len() < int(enc.limit) {
			// success! the incoming event without its ND cache fits into the current chunk
			enc.logError("ND builtins cache dropped from this event to fit under maximum upload size limits. Increase upload size limit or change usage of non-deterministic builtins.")
			enc.incrMetric(logNDBDropCounterName)
			return result, nil
		}

		// The encoder is now in an invalid state and needs to be reset!
		enc.initialize()
		for i := range events {
			// skip last event
			if i == len(events)-1 {
				continue
			}
			// it was a valid chunk until the last event so no need to check anything
			b, err := json.Marshal(&events[i])
			if err != nil {
				return nil, err
			}
			err = enc.appendEvent(b)
			if err != nil {
				return nil, err
			}
		}

		// 3. Try to add the incoming event without its ND cache to a chunk by itself
		tmpEncoder = newChunkEncoder(enc.uncompressedLimit)
		err = tmpEncoder.appendEvent(incomingEventBytesDroppedNDCache)
		if err != nil {
			return nil, err
		}
		if err := tmpEncoder.w.Flush(); err != nil {
			return nil, err
		}
		if tmpEncoder.buf.Len() < int(tmpEncoder.limit) {
			// success! the incoming chunk doesn't have to be dropped and can fit into the next chunk
			enc.logError("ND builtins cache dropped from this event to fit under maximum upload size limits. Increase upload size limit or change usage of non-deterministic builtins.")
			enc.incrMetric(logNDBDropCounterName)
			// flush current encoder buffer to make room for the next event
			if err := enc.writeClose(); err != nil {
				return nil, err
			}
			result = append(result, enc.update()...)

			// write event to empty chunk
			if err := enc.appendEvent(incomingEventBytesDroppedNDCache); err != nil {
				return nil, err
			}

			return result, nil
		}

		// 4. Drop the event, there isn't anything else to be done
		enc.incrMetric(logEncodingFailureCounterName)
		enc.logError("Log encoding failed: received a decision event size (%d) that exceeded the upload_size_limit_bytes (%d). No ND cache to drop.",
			len(incomingEventBytes), enc.limit)
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

	return nil
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
	// don't call enc.reset() because the uncompressed limit shouldn't be updated when forcing the buffer to be emptied
	// the buffer could most likely be underutilized (<90%) and won't be an accurate data point
	return enc.update(), nil
}

func (enc *chunkEncoder) reset() ([][]byte, error) {
	// make sure there aren't any pending writes to get an accurate size
	if err := enc.w.Flush(); err != nil {
		return nil, err
	}

	// Adjust the encoder's uncompressed limit based on the current amount of
	// data written to the underlying buffer. The uncompressed limit decides when to flush a chunk.
	// The uncompressed limit is modified based on the below algorithm:
	// 1) Scale Up: If the current chunk size is within 90% of the user-configured limit, exponentially increase
	// the uncompressed limit. The exponential function is 2^x where x has a minimum value of 1
	// 2) Scale Down: If the current chunk size exceeds the compressed limit, decrease the uncompressed limit and re-encode the
	// decisions in the last chunk.
	// 3) Equilibrium: If the chunk size is between 90% and 100% of the user-configured limit, maintain uncompressed limit value.

	// 1) Scale Up
	if enc.buf.Len() < int(float64(enc.limit)*encCompressedLimitThreshold) {
		enc.incrMetric(encUncompressedLimitScaleUpCounterName)
		enc.incrMetric(encSoftLimitScaleUpCounterName)

		mul := int64(math.Pow(float64(uncompressedLimitBaseFactor), float64(enc.uncompressedLimitScaleUpExponent+1)))
		enc.uncompressedLimit *= mul
		enc.uncompressedLimitScaleUpExponent += uncompressedLimitExponentScaleFactor

		return enc.update(), nil
	}

	// 3) Equilibrium
	if int(enc.limit) > enc.buf.Len() && enc.buf.Len() >= int(float64(enc.limit)*encCompressedLimitThreshold) {
		enc.incrMetric(encUncompressedLimitStableCounterName)
		enc.incrMetric(encSoftLimitStableCounterName)

		enc.uncompressedLimitScaleDownExponent = enc.uncompressedLimitScaleUpExponent
		return enc.update(), nil
	}

	// 2) Scale Down
	if enc.uncompressedLimit > enc.limit {
		enc.incrMetric(encUncompressedLimitScaleDownCounterName)
		enc.incrMetric(encSoftLimitScaleDownCounterName)

		if enc.uncompressedLimitScaleDownExponent < enc.uncompressedLimitScaleUpExponent {
			enc.uncompressedLimitScaleDownExponent = enc.uncompressedLimitScaleUpExponent
		}

		den := int64(math.Pow(float64(uncompressedLimitBaseFactor), float64(enc.uncompressedLimitScaleDownExponent-enc.uncompressedLimitScaleUpExponent+1)))
		enc.uncompressedLimit /= den

		if enc.uncompressedLimitScaleUpExponent > 0 {
			enc.uncompressedLimitScaleUpExponent -= uncompressedLimitExponentScaleFactor
		}
	}

	// if we reach this part of the code it can mean two things:
	// * the uncompressed limit has grown too large and the events need to be split up into multiple chunks
	// * an event has a large ND cache that could be dropped
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

func (enc *chunkEncoder) logError(fmt string, a ...any) {
	if enc.logger != nil {
		enc.logger.Error(fmt, a)
	}
}

func (enc *chunkEncoder) incrMetric(name string) {
	if enc.metrics != nil {
		enc.metrics.Counter(name).Incr()
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
