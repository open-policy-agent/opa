package aescbc

import (
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"sync/atomic"

	"github.com/lestrrat-go/jwx/v3/internal/pool"
)

const (
	NonceSize = 16
)

const defaultBufSize int64 = 256 * 1024 * 1024

var maxBufSize atomic.Int64

func init() {
	SetMaxBufferSize(defaultBufSize)
}

func SetMaxBufferSize(siz int64) {
	if siz <= 0 {
		siz = defaultBufSize
	}
	maxBufSize.Store(siz)
}

func pad(buf []byte, n int) []byte {
	rem := n - len(buf)%n
	if rem == 0 {
		return buf
	}

	bufsiz := len(buf) + rem
	mbs := maxBufSize.Load()
	if int64(bufsiz) > mbs {
		panic(fmt.Errorf("failed to allocate buffer"))
	}
	newbuf := make([]byte, bufsiz)
	copy(newbuf, buf)

	for i := len(buf); i < len(newbuf); i++ {
		newbuf[i] = byte(rem)
	}
	return newbuf
}

// ref. https://github.com/golang/go/blob/c3db64c0f45e8f2d75c5b59401e0fc925701b6f4/src/crypto/tls/conn.go#L279-L324
//
// extractPadding returns, in constant time, the length of the padding to remove
// from the end of payload. It also returns a byte which is equal to 255 if the
// padding was valid and 0 otherwise. See RFC 2246, Section 6.2.3.2.
func extractPadding(payload []byte) (toRemove int, good byte) {
	if len(payload) < 1 {
		return 0, 0
	}

	paddingLen := payload[len(payload)-1]
	t := uint(len(payload)) - uint(paddingLen)
	// if len(payload) > paddingLen then the MSB of t is zero
	good = byte(int32(^t) >> 31)

	// The maximum possible padding length plus the actual length field
	toCheck := 256
	// The length of the padded data is public, so we can use an if here
	if toCheck > len(payload) {
		toCheck = len(payload)
	}

	for i := 1; i <= toCheck; i++ {
		t := uint(paddingLen) - uint(i)
		// if i <= paddingLen then the MSB of t is zero
		mask := byte(int32(^t) >> 31)
		b := payload[len(payload)-i]
		good &^= mask&paddingLen ^ mask&b
	}

	// We AND together the bits of good and replicate the result across
	// all the bits.
	good &= good << 4
	good &= good << 2
	good &= good << 1
	good = uint8(int8(good) >> 7)

	// Zero the padding length on error. This ensures any unchecked bytes
	// are included in the MAC. Otherwise, an attacker that could
	// distinguish MAC failures from padding failures could mount an attack
	// similar to POODLE in SSL 3.0: given a good ciphertext that uses a
	// full block's worth of padding, replace the final block with another
	// block. If the MAC check passed but the padding check failed, the
	// last byte of that block decrypted to the block size.
	//
	// See also macAndPaddingGood logic below.
	paddingLen &= good

	toRemove = int(paddingLen)
	return
}

type Hmac struct {
	blockCipher  cipher.Block
	hash         func() hash.Hash
	keysize      int
	tagsize      int
	integrityKey []byte
}

type BlockCipherFunc func([]byte) (cipher.Block, error)

func New(key []byte, f BlockCipherFunc) (hmac *Hmac, err error) {
	keysize := len(key) / 2
	ikey := key[:keysize]
	ekey := key[keysize:]

	bc, ciphererr := f(ekey)
	if ciphererr != nil {
		err = fmt.Errorf(`failed to execute block cipher function: %w`, ciphererr)
		return
	}

	var hfunc func() hash.Hash
	switch keysize {
	case 16:
		hfunc = sha256.New
	case 24:
		hfunc = sha512.New384
	case 32:
		hfunc = sha512.New
	default:
		return nil, fmt.Errorf("unsupported key size %d", keysize)
	}

	return &Hmac{
		blockCipher:  bc,
		hash:         hfunc,
		integrityKey: ikey,
		keysize:      keysize,
		tagsize:      keysize, // NonceSize,
		// While investigating GH #207, I stumbled upon another problem where
		// the computed tags don't match on decrypt. After poking through the
		// code using a bunch of debug statements, I've finally found out that
		// tagsize = keysize makes the whole thing work.
	}, nil
}

// NonceSize fulfills the crypto.AEAD interface
func (c Hmac) NonceSize() int {
	return NonceSize
}

// Overhead fulfills the crypto.AEAD interface
func (c Hmac) Overhead() int {
	return c.blockCipher.BlockSize() + c.tagsize
}

func (c Hmac) ComputeAuthTag(aad, nonce, ciphertext []byte) ([]byte, error) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(len(aad)*8))

	h := hmac.New(c.hash, c.integrityKey)

	// compute the tag
	// no need to check errors because Write never returns an error: https://pkg.go.dev/hash#Hash
	//
	// > Write (via the embedded io.Writer interface) adds more data to the running hash.
	// > It never returns an error.
	h.Write(aad)
	h.Write(nonce)
	h.Write(ciphertext)
	h.Write(buf[:])
	s := h.Sum(nil)
	return s[:c.tagsize], nil
}

func ensureSize(dst []byte, n int) []byte {
	// if the dst buffer has enough length just copy the relevant parts to it.
	// Otherwise create a new slice that's big enough, and operate on that
	// Note: I think go-jose has a bug in that it checks for cap(), but not len().
	ret := dst
	if diff := n - len(dst); diff > 0 {
		// dst is not big enough
		ret = make([]byte, n)
		copy(ret, dst)
	}
	return ret
}

// Seal fulfills the crypto.AEAD interface
func (c Hmac) Seal(dst, nonce, plaintext, data []byte) []byte {
	ctlen := len(plaintext)
	bufsiz := ctlen + c.Overhead()
	mbs := maxBufSize.Load()

	if int64(bufsiz) > mbs {
		panic(fmt.Errorf("failed to allocate buffer"))
	}
	ciphertext := make([]byte, bufsiz)[:ctlen]
	copy(ciphertext, plaintext)
	ciphertext = pad(ciphertext, c.blockCipher.BlockSize())

	cbc := cipher.NewCBCEncrypter(c.blockCipher, nonce)
	cbc.CryptBlocks(ciphertext, ciphertext)

	authtag, err := c.ComputeAuthTag(data, nonce, ciphertext)
	if err != nil {
		// Hmac implements cipher.AEAD interface. Seal can't return error.
		// But currently it never reach here because of Hmac.ComputeAuthTag doesn't return error.
		panic(fmt.Errorf("failed to seal on hmac: %v", err))
	}

	retlen := len(dst) + len(ciphertext) + len(authtag)

	ret := ensureSize(dst, retlen)
	out := ret[len(dst):]
	n := copy(out, ciphertext)
	copy(out[n:], authtag)

	return ret
}

// Open fulfills the crypto.AEAD interface
func (c Hmac) Open(dst, nonce, ciphertext, data []byte) ([]byte, error) {
	if len(ciphertext) < c.keysize {
		return nil, fmt.Errorf(`invalid ciphertext (too short)`)
	}

	tagOffset := len(ciphertext) - c.tagsize
	if tagOffset%c.blockCipher.BlockSize() != 0 {
		return nil, fmt.Errorf(
			"invalid ciphertext (invalid length: %d %% %d != 0)",
			tagOffset,
			c.blockCipher.BlockSize(),
		)
	}
	tag := ciphertext[tagOffset:]
	ciphertext = ciphertext[:tagOffset]

	expectedTag, err := c.ComputeAuthTag(data, nonce, ciphertext[:tagOffset])
	if err != nil {
		return nil, fmt.Errorf(`failed to compute auth tag: %w`, err)
	}

	cbc := cipher.NewCBCDecrypter(c.blockCipher, nonce)
	buf := pool.ByteSlice().GetCapacity(tagOffset)
	defer pool.ByteSlice().Put(buf)
	buf = buf[:tagOffset]

	cbc.CryptBlocks(buf, ciphertext)

	toRemove, good := extractPadding(buf)
	cmp := subtle.ConstantTimeCompare(expectedTag, tag) & int(good)
	if cmp != 1 {
		return nil, errors.New(`invalid ciphertext`)
	}

	plaintext := buf[:len(buf)-toRemove]
	ret := ensureSize(dst, len(plaintext))
	out := ret[len(dst):]
	copy(out, plaintext)
	return ret, nil
}
