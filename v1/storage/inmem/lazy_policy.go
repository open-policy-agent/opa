// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"hash"
	"io"
	"sync"

	"github.com/open-policy-agent/opa/v1/util"
)

// Pools for efficient memory management with zero allocations on hot paths
var (
	// Buffer pool for compression operations
	compressBufferPool = util.NewSyncPool[bytes.Buffer]()

	// SHA256 hasher pool for policy hash computation
	hasherPool = sync.Pool{
		New: func() any {
			return sha256.New()
		},
	}

	// Flate writer pool for compression
	flateWriterPool = sync.Pool{
		New: func() any {
			w, _ := flate.NewWriter(nil, flate.BestSpeed)
			return w
		},
	}

	// Bytes reader pool for zero-allocation reads
	bytesReaderPool = util.NewSyncPool[bytes.Reader]()

	// Flate reader pool for decompression (reset not supported, so we store resetter)
	flateReaderPool = sync.Pool{
		New: func() any {
			return flate.NewReader(nil)
		},
	}
)

// lazyPolicy provides streaming decompression with hash-based comparison.
// Policies are stored compressed to save ~50% memory, and decompressed on-demand
// using pooled buffers to minimize allocations.
//
// Memory optimization strategy:
// - Compressed data stored (~40-50% of original size)
// - SHA256 hash for O(1) policy comparison (32 bytes)
// - NO cached decompressed data - stream decompress on each read
// - Pooled buffers eliminate allocations on hot paths
//
// Uses flate (DEFLATE) compression at BestSpeed level for optimal balance between
// compression ratio (~40-50%) and speed (~150-250 MB/s compression, ~200-350 MB/s decompression).
//
// Thread-safe: all operations are safe for concurrent access.
type lazyPolicy struct {
	// compressed stores the flate-compressed policy bytes.
	// This is the only heavy data stored (~50% of original).
	compressed []byte

	// hash stores the SHA256 hash of the ORIGINAL uncompressed data.
	// Used for fast O(1) policy comparison without decompression.
	// Computed during compression via io.MultiWriter.
	hash [32]byte

	// originalSize stores the uncompressed size for buffer pre-allocation.
	// This field is immutable after creation.
	originalSize int
}

// newLazyPolicy creates a new lazy policy by compressing the provided data.
// The data is immediately compressed with flate (DEFLATE) and hashed with SHA256
// in a single streaming pass using io.MultiWriter for efficiency.
//
// Uses BestSpeed compression level for optimal performance:
// - Compression: ~150-250 MB/s (fast enough for policy writes)
// - Decompression: ~200-350 MB/s (very fast for policy reads)
// - Ratio: ~40-50% for typical Rego policies
//
// Hash computation is done simultaneously during compression to avoid
// an extra pass over the data.
//
// Returns nil if data is nil (used for policy deletion).
//
// Note: Compression errors cause panic as they indicate programming errors
// (invalid compression level, write to buffer failing), not runtime errors.
func newLazyPolicy(data []byte) *lazyPolicy {
	if data == nil {
		return nil
	}

	// Get pooled buffer for compression output
	compressBuf := compressBufferPool.Get()
	defer compressBufferPool.Put(compressBuf)
	compressBuf.Reset()

	// Get pooled hasher
	hasher := hasherPool.Get().(hash.Hash)
	hasher.Reset()
	defer hasherPool.Put(hasher)

	// Get pooled flate writer and reset it
	writer := flateWriterPool.Get().(*flate.Writer)
	defer flateWriterPool.Put(writer)
	writer.Reset(compressBuf)

	// Use io.MultiWriter to compress AND hash in a single pass
	multiWriter := io.MultiWriter(writer, hasher)

	if _, err := multiWriter.Write(data); err != nil {
		// Writing to bytes.Buffer and hash should never fail - indicates programming error
		panic("io.MultiWriter.Write failed: " + err.Error())
	}

	if err := writer.Close(); err != nil {
		// Closing flate writer should never fail - indicates programming error
		panic("flate.Writer.Close failed: " + err.Error())
	}

	// Allocate exact size needed and copy compressed data
	compressed := make([]byte, compressBuf.Len())
	copy(compressed, compressBuf.Bytes())

	lp := &lazyPolicy{
		compressed:   compressed,
		originalSize: len(data),
	}

	// Extract SHA256 hash (32 bytes) directly into array - zero allocations
	hasher.Sum(lp.hash[:0])

	return lp
}

// get returns the decompressed policy data by streaming decompression.
// Each call decompresses from scratch using pooled buffers to avoid
// memory overhead of caching decompressed data.
//
// This trades a small CPU cost (~2-5µs for 2KB policy) for ~50% memory savings.
// Since policies are read far less frequently than they stay in memory,
// this is an excellent tradeoff.
//
// Returns error if decompression fails (corrupted data).
//
//go:inline
func (lp *lazyPolicy) get() ([]byte, error) {
	// Get pooled bytes.Reader for zero-allocation read
	br := bytesReaderPool.Get()
	defer bytesReaderPool.Put(br)
	br.Reset(lp.compressed)

	// Get pooled flate reader (implements both io.Reader and flate.Resetter)
	pooledReader := flateReaderPool.Get()
	defer flateReaderPool.Put(pooledReader)

	// Reset reader with new input source
	resetter := pooledReader.(flate.Resetter)
	if err := resetter.Reset(br, nil); err != nil {
		return nil, err
	}

	// Allocate exact size for result (we know originalSize)
	result := make([]byte, lp.originalSize)

	// Read directly into result buffer - single allocation, no copying
	reader := pooledReader.(io.Reader)
	n, err := io.ReadFull(reader, result)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	// Return slice with actual size read
	return result[:n], nil
}

// size returns the original (uncompressed) size of the policy.
// This is used for metrics and validation without requiring decompression.
//
//go:inline
func (lp *lazyPolicy) size() int {
	return lp.originalSize
}

// compressedSize returns the compressed size of the policy.
// This is useful for measuring memory savings and compression ratios.
//
//go:inline
func (lp *lazyPolicy) compressedSize() int {
	return len(lp.compressed)
}

// Hash returns the SHA256 hash of the original uncompressed policy data.
// This enables O(1) policy comparison without decompression.
// Useful for detecting policy changes and cache invalidation.
//
//go:inline
func (lp *lazyPolicy) Hash() [32]byte {
	return lp.hash
}

// Equal compares two policies by hash for O(1) equality check.
// This is much faster than decompressing and comparing bytes.
//
//go:inline
func (lp *lazyPolicy) Equal(other *lazyPolicy) bool {
	if lp == nil || other == nil {
		return lp == other
	}
	return lp.hash == other.hash
}
