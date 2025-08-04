package jwebb

import (
	"crypto/cipher"
	"crypto/subtle"
	"encoding/binary"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
)

var keywrapDefaultIV = []byte{0xa6, 0xa6, 0xa6, 0xa6, 0xa6, 0xa6, 0xa6, 0xa6}

func Wrap(kek cipher.Block, cek []byte) ([]byte, error) {
	if len(cek)%tokens.KeywrapBlockSize != 0 {
		return nil, fmt.Errorf(`keywrap input must be %d byte blocks`, tokens.KeywrapBlockSize)
	}

	n := len(cek) / tokens.KeywrapChunkLen
	r := make([][]byte, n)

	for i := range n {
		r[i] = make([]byte, tokens.KeywrapChunkLen)
		copy(r[i], cek[i*tokens.KeywrapChunkLen:])
	}

	buffer := pool.ByteSlice().GetCapacity(tokens.KeywrapChunkLen * 2)
	defer pool.ByteSlice().Put(buffer)
	// the byte slice has the capacity, but len is 0
	buffer = buffer[:tokens.KeywrapChunkLen*2]

	tBytes := pool.ByteSlice().GetCapacity(tokens.KeywrapChunkLen)
	defer pool.ByteSlice().Put(tBytes)
	// the byte slice has the capacity, but len is 0
	tBytes = tBytes[:tokens.KeywrapChunkLen]

	copy(buffer, keywrapDefaultIV)

	for t := range tokens.KeywrapRounds * n {
		copy(buffer[tokens.KeywrapChunkLen:], r[t%n])

		kek.Encrypt(buffer, buffer)

		binary.BigEndian.PutUint64(tBytes, uint64(t+1))

		for i := range tokens.KeywrapChunkLen {
			buffer[i] = buffer[i] ^ tBytes[i]
		}
		copy(r[t%n], buffer[tokens.KeywrapChunkLen:])
	}

	out := make([]byte, (n+1)*tokens.KeywrapChunkLen)
	copy(out, buffer[:tokens.KeywrapChunkLen])
	for i := range r {
		copy(out[(i+1)*tokens.KeywrapBlockSize:], r[i])
	}

	return out, nil
}

func Unwrap(block cipher.Block, ciphertxt []byte) ([]byte, error) {
	if len(ciphertxt)%tokens.KeywrapChunkLen != 0 {
		return nil, fmt.Errorf(`keyunwrap input must be %d byte blocks`, tokens.KeywrapChunkLen)
	}

	n := (len(ciphertxt) / tokens.KeywrapChunkLen) - 1
	r := make([][]byte, n)

	for i := range r {
		r[i] = make([]byte, tokens.KeywrapChunkLen)
		copy(r[i], ciphertxt[(i+1)*tokens.KeywrapChunkLen:])
	}

	buffer := pool.ByteSlice().GetCapacity(tokens.KeywrapChunkLen * 2)
	defer pool.ByteSlice().Put(buffer)
	// the byte slice has the capacity, but len is 0
	buffer = buffer[:tokens.KeywrapChunkLen*2]

	tBytes := pool.ByteSlice().GetCapacity(tokens.KeywrapChunkLen)
	defer pool.ByteSlice().Put(tBytes)
	// the byte slice has the capacity, but len is 0
	tBytes = tBytes[:tokens.KeywrapChunkLen]

	copy(buffer[:tokens.KeywrapChunkLen], ciphertxt[:tokens.KeywrapChunkLen])

	for t := tokens.KeywrapRounds*n - 1; t >= 0; t-- {
		binary.BigEndian.PutUint64(tBytes, uint64(t+1))

		for i := range tokens.KeywrapChunkLen {
			buffer[i] = buffer[i] ^ tBytes[i]
		}
		copy(buffer[tokens.KeywrapChunkLen:], r[t%n])

		block.Decrypt(buffer, buffer)

		copy(r[t%n], buffer[tokens.KeywrapChunkLen:])
	}

	if subtle.ConstantTimeCompare(buffer[:tokens.KeywrapChunkLen], keywrapDefaultIV) == 0 {
		return nil, fmt.Errorf(`key unwrap: failed to unwrap key`)
	}

	out := make([]byte, n*tokens.KeywrapChunkLen)
	for i := range r {
		copy(out[i*tokens.KeywrapChunkLen:], r[i])
	}

	return out, nil
}
