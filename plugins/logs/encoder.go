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

	"github.com/open-policy-agent/opa/metrics"
)

const (
	encHardLimitThreshold            = 0.9
	softLimitBaseFactor              = 2
	softLimitExponentScaleFactor     = 0.2
	encSoftLimitScaleUpCounterName   = "enc_soft_limit_scale_up"
	encSoftLimitScaleDownCounterName = "enc_soft_limit_scale_down"
	encSoftLimitStableCounterName    = "enc_soft_limit_stable"
)

// chunkEncoder implements log buffer chunking and compression. Log events are
// written to the encoder and the encoder outputs chunks that are fit to the
// configured limit.
type chunkEncoder struct {
	limit                      int64
	softLimit                  int64
	softLimitScaleUpExponent   float64
	softLimitScaleDownExponent float64
	bytesWritten               int
	buf                        *bytes.Buffer
	w                          *gzip.Writer
	metrics                    metrics.Metrics
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

	bs := buf.Bytes()

	if len(bs) == 0 {
		return nil, nil
	} else if int64(len(bs)+2) > enc.limit {
		return nil, fmt.Errorf("upload chunk size (%d) exceeds upload_size_limit_bytes (%d)",
			int64(len(bs)+2), enc.limit)
	}

	if int64(len(bs)+enc.bytesWritten+1) > enc.softLimit {
		if err := enc.writeClose(); err != nil {
			return nil, err
		}

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

	n, err := enc.w.Write(bs)
	if err != nil {
		return nil, err
	}

	enc.bytesWritten += n
	return
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

	if enc.buf.Len() < int(float64(enc.limit)*encHardLimitThreshold) {
		if enc.metrics != nil {
			enc.metrics.Counter(encSoftLimitScaleUpCounterName).Incr()
		}

		mul := int64(math.Pow(float64(softLimitBaseFactor), float64(enc.softLimitScaleUpExponent+1)))
		enc.softLimit *= mul
		enc.softLimitScaleUpExponent += softLimitExponentScaleFactor
		return enc.update(), nil
	}

	if int(enc.limit) > enc.buf.Len() && enc.buf.Len() >= int(float64(enc.limit)*encHardLimitThreshold) {
		if enc.metrics != nil {
			enc.metrics.Counter(encSoftLimitStableCounterName).Incr()
		}

		enc.softLimitScaleDownExponent = enc.softLimitScaleUpExponent
		return enc.update(), nil
	}

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
	for _, event := range events {
		chunk, err := enc.Write(event)
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
	gr.Close()

	return events, nil
}
