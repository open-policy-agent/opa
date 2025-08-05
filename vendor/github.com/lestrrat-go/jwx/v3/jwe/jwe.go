//go:generate ../tools/cmd/genjwe.sh

// Package jwe implements JWE as described in https://tools.ietf.org/html/rfc7516
package jwe

// #region imports
import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/aescbc"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/content_crypt"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/keygen"
)

// #region globals

var muSettings sync.RWMutex
var maxPBES2Count = 10000
var maxDecompressBufferSize int64 = 10 * 1024 * 1024 // 10MB

func Settings(options ...GlobalOption) {
	muSettings.Lock()
	defer muSettings.Unlock()
	for _, option := range options {
		switch option.Ident() {
		case identMaxPBES2Count{}:
			if err := option.Value(&maxPBES2Count); err != nil {
				panic(fmt.Sprintf("jwe.Settings: value for option WithMaxPBES2Count must be an int: %s", err))
			}
		case identMaxDecompressBufferSize{}:
			if err := option.Value(&maxDecompressBufferSize); err != nil {
				panic(fmt.Sprintf("jwe.Settings: value for option WithMaxDecompressBufferSize must be an int64: %s", err))
			}
		case identCBCBufferSize{}:
			var v int64
			if err := option.Value(&v); err != nil {
				panic(fmt.Sprintf("jwe.Settings: value for option WithCBCBufferSize must be an int64: %s", err))
			}
			aescbc.SetMaxBufferSize(v)
		}
	}
}

const (
	fmtInvalid = iota
	fmtCompact
	fmtJSON
	fmtJSONPretty
	fmtMax
)

var _ = fmtInvalid
var _ = fmtMax

var registry = json.NewRegistry()

type recipientBuilder struct {
	alg     jwa.KeyEncryptionAlgorithm
	key     any
	headers Headers
}

func (b *recipientBuilder) Build(r Recipient, cek []byte, calg jwa.ContentEncryptionAlgorithm, _ *content_crypt.Generic) ([]byte, error) {
	// we need the raw key for later use
	rawKey := b.key

	var keyID string
	if ke, ok := b.key.(KeyEncrypter); ok {
		if kider, ok := ke.(KeyIDer); ok {
			if v, ok := kider.KeyID(); ok {
				keyID = v
			}
		}
	} else if jwkKey, ok := b.key.(jwk.Key); ok {
		// Meanwhile, grab the kid as well
		if v, ok := jwkKey.KeyID(); ok {
			keyID = v
		}

		var raw any
		if err := jwk.Export(jwkKey, &raw); err != nil {
			return nil, fmt.Errorf(`jwe.Encrypt: recipientBuilder: failed to retrieve raw key out of %T: %w`, b.key, err)
		}

		rawKey = raw
	}

	// Extract ECDH-ES specific parameters if needed
	var apu, apv []byte
	if b.headers != nil {
		if val, ok := b.headers.AgreementPartyUInfo(); ok {
			apu = val
		}
		if val, ok := b.headers.AgreementPartyVInfo(); ok {
			apv = val
		}
	}

	// Create the encrypter using the new jwebb pattern
	enc, err := newEncrypter(b.alg, calg, b.key, rawKey, apu, apv)
	if err != nil {
		return nil, fmt.Errorf(`jwe.Encrypt: recipientBuilder: failed to create encrypter: %w`, err)
	}

	if hdrs := b.headers; hdrs != nil {
		_ = r.SetHeaders(hdrs)
	}

	if err := r.Headers().Set(AlgorithmKey, b.alg); err != nil {
		return nil, fmt.Errorf(`failed to set header: %w`, err)
	}

	if keyID != "" {
		if err := r.Headers().Set(KeyIDKey, keyID); err != nil {
			return nil, fmt.Errorf(`failed to set header: %w`, err)
		}
	}

	var rawCEK []byte
	enckey, err := enc.EncryptKey(cek)
	if err != nil {
		return nil, fmt.Errorf(`failed to encrypt key: %w`, err)
	}
	if b.alg == jwa.ECDH_ES() || b.alg == jwa.DIRECT() {
		rawCEK = enckey.Bytes()
	} else {
		if err := r.SetEncryptedKey(enckey.Bytes()); err != nil {
			return nil, fmt.Errorf(`failed to set encrypted key: %w`, err)
		}
	}

	if hp, ok := enckey.(populater); ok {
		if err := hp.Populate(r.Headers()); err != nil {
			return nil, fmt.Errorf(`failed to populate: %w`, err)
		}
	}

	return rawCEK, nil
}

// Encrypt generates a JWE message for the given payload and returns
// it in serialized form, which can be in either compact or
// JSON format. Default is compact.
//
// You must pass at least one key to `jwe.Encrypt()` by using `jwe.WithKey()`
// option.
//
//	jwe.Encrypt(payload, jwe.WithKey(alg, key))
//	jwe.Encrypt(payload, jws.WithJSON(), jws.WithKey(alg1, key1), jws.WithKey(alg2, key2))
//
// Note that in the second example the `jws.WithJSON()` option is
// specified as well. This is because the compact serialization
// format does not support multiple recipients, and users must
// specifically ask for the JSON serialization format.
//
// Read the documentation for `jwe.WithKey()` to learn more about the
// possible values that can be used for `alg` and `key`.
//
// Look for options that return `jwe.EncryptOption` or `jws.EncryptDecryptOption`
// for a complete list of options that can be passed to this function.
func Encrypt(payload []byte, options ...EncryptOption) ([]byte, error) {
	ec := encryptContextPool.Get()
	defer encryptContextPool.Put(ec)
	if err := ec.ProcessOptions(options); err != nil {
		return nil, encryptError{fmt.Errorf(`jwe.Encrypt: failed to process options: %w`, err)}
	}
	ret, err := ec.EncryptMessage(payload, nil)
	if err != nil {
		return nil, encryptError{fmt.Errorf(`jwe.Encrypt: %w`, err)}
	}
	return ret, nil
}

// EncryptStatic is exactly like Encrypt, except it accepts a static
// content encryption key (CEK). It is separated out from the main
// Encrypt function such that the latter does not accidentally use a static
// CEK.
//
// DO NOT attempt to use this function unless you completely understand the
// security implications to using static CEKs. You have been warned.
//
// This function is currently considered EXPERIMENTAL, and is subject to
// future changes across minor/micro versions.
func EncryptStatic(payload, cek []byte, options ...EncryptOption) ([]byte, error) {
	if len(cek) <= 0 {
		return nil, encryptError{fmt.Errorf(`jwe.EncryptStatic: empty CEK`)}
	}
	ec := encryptContextPool.Get()
	defer encryptContextPool.Put(ec)
	if err := ec.ProcessOptions(options); err != nil {
		return nil, encryptError{fmt.Errorf(`jwe.EncryptStatic: failed to process options: %w`, err)}
	}
	ret, err := ec.EncryptMessage(payload, cek)
	if err != nil {
		return nil, encryptError{fmt.Errorf(`jwe.EncryptStatic: %w`, err)}
	}
	return ret, nil
}

// decryptContext holds the state during JWE decryption, similar to JWS verifyContext
type decryptContext struct {
	keyProviders            []KeyProvider
	keyUsed                 any
	cek                     *[]byte
	dst                     *Message
	maxDecompressBufferSize int64
	//nolint:containedctx
	ctx context.Context
}

var decryptContextPool = pool.New(allocDecryptContext, freeDecryptContext)

func allocDecryptContext() *decryptContext {
	return &decryptContext{
		ctx: context.Background(),
	}
}

func freeDecryptContext(dc *decryptContext) *decryptContext {
	dc.keyProviders = dc.keyProviders[:0]
	dc.keyUsed = nil
	dc.cek = nil
	dc.dst = nil
	dc.maxDecompressBufferSize = 0
	dc.ctx = context.Background()
	return dc
}

func (dc *decryptContext) ProcessOptions(options []DecryptOption) error {
	// Set default max decompress buffer size
	muSettings.RLock()
	dc.maxDecompressBufferSize = maxDecompressBufferSize
	muSettings.RUnlock()

	for _, option := range options {
		switch option.Ident() {
		case identMessage{}:
			if err := option.Value(&dc.dst); err != nil {
				return fmt.Errorf("jwe.decrypt: WithMessage must be a *jwe.Message: %w", err)
			}
		case identKeyProvider{}:
			var kp KeyProvider
			if err := option.Value(&kp); err != nil {
				return fmt.Errorf("jwe.decrypt: WithKeyProvider must be a KeyProvider: %w", err)
			}
			dc.keyProviders = append(dc.keyProviders, kp)
		case identKeyUsed{}:
			if err := option.Value(&dc.keyUsed); err != nil {
				return fmt.Errorf("jwe.decrypt: WithKeyUsed must be an any: %w", err)
			}
		case identKey{}:
			var pair *withKey
			if err := option.Value(&pair); err != nil {
				return fmt.Errorf("jwe.decrypt: WithKey must be a *withKey: %w", err)
			}
			alg, ok := pair.alg.(jwa.KeyEncryptionAlgorithm)
			if !ok {
				return fmt.Errorf("jwe.decrypt: WithKey() option must be specified using jwa.KeyEncryptionAlgorithm (got %T)", pair.alg)
			}
			dc.keyProviders = append(dc.keyProviders, &staticKeyProvider{alg: alg, key: pair.key})
		case identCEK{}:
			if err := option.Value(&dc.cek); err != nil {
				return fmt.Errorf("jwe.decrypt: WithCEK must be a *[]byte: %w", err)
			}
		case identMaxDecompressBufferSize{}:
			if err := option.Value(&dc.maxDecompressBufferSize); err != nil {
				return fmt.Errorf("jwe.decrypt: WithMaxDecompressBufferSize must be int64: %w", err)
			}
		case identContext{}:
			if err := option.Value(&dc.ctx); err != nil {
				return fmt.Errorf("jwe.decrypt: WithContext must be a context.Context: %w", err)
			}
		}
	}

	if len(dc.keyProviders) < 1 {
		return fmt.Errorf(`jwe.Decrypt: no key providers have been provided (see jwe.WithKey(), jwe.WithKeySet(), and jwe.WithKeyProvider()`)
	}

	return nil
}

func (dc *decryptContext) DecryptMessage(buf []byte) ([]byte, error) {
	msg, err := parseJSONOrCompact(buf, true)
	if err != nil {
		return nil, fmt.Errorf(`failed to parse buffer for Decrypt: %w`, err)
	}

	// Process things that are common to the message
	h, err := msg.protectedHeaders.Clone()
	if err != nil {
		return nil, fmt.Errorf(`failed to copy protected headers: %w`, err)
	}
	h, err = h.Merge(msg.unprotectedHeaders)
	if err != nil {
		return nil, fmt.Errorf(`failed to merge headers for message decryption: %w`, err)
	}

	var aad []byte
	if aadContainer := msg.authenticatedData; aadContainer != nil {
		aad = base64.Encode(aadContainer)
	}

	var computedAad []byte
	if len(msg.rawProtectedHeaders) > 0 {
		computedAad = msg.rawProtectedHeaders
	} else {
		// this is probably not required once msg.Decrypt is deprecated
		var err error
		computedAad, err = msg.protectedHeaders.Encode()
		if err != nil {
			return nil, fmt.Errorf(`failed to encode protected headers: %w`, err)
		}
	}

	// for each recipient, attempt to match the key providers
	// if we have no recipients, pretend like we only have one
	recipients := msg.recipients
	if len(recipients) == 0 {
		r := NewRecipient()
		if err := r.SetHeaders(msg.protectedHeaders); err != nil {
			return nil, fmt.Errorf(`failed to set headers to recipient: %w`, err)
		}
		recipients = append(recipients, r)
	}

	errs := make([]error, 0, len(recipients))
	for _, recipient := range recipients {
		decrypted, err := dc.tryRecipient(msg, recipient, h, aad, computedAad)
		if err != nil {
			errs = append(errs, recipientError{err})
			continue
		}
		if dc.dst != nil {
			*dc.dst = *msg
			dc.dst.rawProtectedHeaders = nil
			dc.dst.storeProtectedHeaders = false
		}
		return decrypted, nil
	}
	return nil, fmt.Errorf(`failed to decrypt any of the recipients: %w`, errors.Join(errs...))
}

func (dc *decryptContext) tryRecipient(msg *Message, recipient Recipient, protectedHeaders Headers, aad, computedAad []byte) ([]byte, error) {
	var tried int
	var lastError error
	for i, kp := range dc.keyProviders {
		var sink algKeySink
		if err := kp.FetchKeys(dc.ctx, &sink, recipient, msg); err != nil {
			return nil, fmt.Errorf(`key provider %d failed: %w`, i, err)
		}

		for _, pair := range sink.list {
			tried++
			// alg is converted here because pair.alg is of type jwa.KeyAlgorithm.
			// this may seem ugly, but we're trying to avoid declaring separate
			// structs for `alg jwa.KeyEncryptionAlgorithm` and `alg jwa.SignatureAlgorithm`
			//nolint:forcetypeassert
			alg := pair.alg.(jwa.KeyEncryptionAlgorithm)
			key := pair.key

			decrypted, err := dc.decryptContent(msg, alg, key, recipient, protectedHeaders, aad, computedAad)
			if err != nil {
				lastError = err
				continue
			}

			if dc.keyUsed != nil {
				if err := blackmagic.AssignIfCompatible(dc.keyUsed, key); err != nil {
					return nil, fmt.Errorf(`failed to assign used key (%T) to %T: %w`, key, dc.keyUsed, err)
				}
			}
			return decrypted, nil
		}
	}
	return nil, fmt.Errorf(`jwe.Decrypt: tried %d keys, but failed to match any of the keys with recipient (last error = %s)`, tried, lastError)
}

func (dc *decryptContext) decryptContent(msg *Message, alg jwa.KeyEncryptionAlgorithm, key any, recipient Recipient, protectedHeaders Headers, aad, computedAad []byte) ([]byte, error) {
	if jwkKey, ok := key.(jwk.Key); ok {
		var raw any
		if err := jwk.Export(jwkKey, &raw); err != nil {
			return nil, fmt.Errorf(`failed to retrieve raw key from %T: %w`, key, err)
		}
		key = raw
	}

	ce, ok := msg.protectedHeaders.ContentEncryption()
	if !ok {
		return nil, fmt.Errorf(`jwe.Decrypt: failed to retrieve content encryption algorithm from protected headers`)
	}
	dec := newDecrypter(alg, ce, key).
		AuthenticatedData(aad).
		ComputedAuthenticatedData(computedAad).
		InitializationVector(msg.initializationVector).
		Tag(msg.tag).
		CEK(dc.cek)

	if v, ok := recipient.Headers().Algorithm(); !ok || v != alg {
		// algorithms don't match
		return nil, fmt.Errorf(`jwe.Decrypt: key (%q) and recipient (%q) algorithms do not match`, alg, v)
	}

	h2, err := protectedHeaders.Clone()
	if err != nil {
		return nil, fmt.Errorf(`jwe.Decrypt: failed to copy headers (1): %w`, err)
	}

	h2, err = h2.Merge(recipient.Headers())
	if err != nil {
		return nil, fmt.Errorf(`failed to copy headers (2): %w`, err)
	}

	switch alg {
	case jwa.ECDH_ES(), jwa.ECDH_ES_A128KW(), jwa.ECDH_ES_A192KW(), jwa.ECDH_ES_A256KW():
		var epk any
		if err := h2.Get(EphemeralPublicKeyKey, &epk); err != nil {
			return nil, fmt.Errorf(`failed to get 'epk' field: %w`, err)
		}
		switch epk := epk.(type) {
		case jwk.ECDSAPublicKey:
			var pubkey ecdsa.PublicKey
			if err := jwk.Export(epk, &pubkey); err != nil {
				return nil, fmt.Errorf(`failed to get public key: %w`, err)
			}
			dec.PublicKey(&pubkey)
		case jwk.OKPPublicKey:
			var pubkey any
			if err := jwk.Export(epk, &pubkey); err != nil {
				return nil, fmt.Errorf(`failed to get public key: %w`, err)
			}
			dec.PublicKey(pubkey)
		default:
			return nil, fmt.Errorf("unexpected 'epk' type %T for alg %s", epk, alg)
		}

		if apu, ok := h2.AgreementPartyUInfo(); ok && len(apu) > 0 {
			dec.AgreementPartyUInfo(apu)
		}
		if apv, ok := h2.AgreementPartyVInfo(); ok && len(apv) > 0 {
			dec.AgreementPartyVInfo(apv)
		}
	case jwa.A128GCMKW(), jwa.A192GCMKW(), jwa.A256GCMKW():
		var ivB64 string
		if err := h2.Get(InitializationVectorKey, &ivB64); err == nil {
			iv, err := base64.DecodeString(ivB64)
			if err != nil {
				return nil, fmt.Errorf(`failed to b64-decode 'iv': %w`, err)
			}
			dec.KeyInitializationVector(iv)
		}
		var tagB64 string
		if err := h2.Get(TagKey, &tagB64); err == nil {
			tag, err := base64.DecodeString(tagB64)
			if err != nil {
				return nil, fmt.Errorf(`failed to b64-decode 'tag': %w`, err)
			}
			dec.KeyTag(tag)
		}
	case jwa.PBES2_HS256_A128KW(), jwa.PBES2_HS384_A192KW(), jwa.PBES2_HS512_A256KW():
		var saltB64 string
		if err := h2.Get(SaltKey, &saltB64); err != nil {
			return nil, fmt.Errorf(`failed to get %q field`, SaltKey)
		}

		// check if WithUseNumber is effective, because it will change the
		// type of the underlying value (#1140)
		var countFlt float64
		if json.UseNumber() {
			var count json.Number
			if err := h2.Get(CountKey, &count); err != nil {
				return nil, fmt.Errorf(`failed to get %q field`, CountKey)
			}
			v, err := count.Float64()
			if err != nil {
				return nil, fmt.Errorf("failed to convert 'p2c' to float64: %w", err)
			}
			countFlt = v
		} else {
			var count float64
			if err := h2.Get(CountKey, &count); err != nil {
				return nil, fmt.Errorf(`failed to get %q field`, CountKey)
			}
			countFlt = count
		}

		muSettings.RLock()
		maxCount := maxPBES2Count
		muSettings.RUnlock()
		if countFlt > float64(maxCount) {
			return nil, fmt.Errorf("invalid 'p2c' value")
		}
		salt, err := base64.DecodeString(saltB64)
		if err != nil {
			return nil, fmt.Errorf(`failed to b64-decode 'salt': %w`, err)
		}
		dec.KeySalt(salt)
		dec.KeyCount(int(countFlt))
	}

	plaintext, err := dec.Decrypt(recipient, msg.cipherText, msg)
	if err != nil {
		return nil, fmt.Errorf(`jwe.Decrypt: decryption failed: %w`, err)
	}

	if v, ok := h2.Compression(); ok && v == jwa.Deflate() {
		buf, err := uncompress(plaintext, dc.maxDecompressBufferSize)
		if err != nil {
			return nil, fmt.Errorf(`jwe.Derypt: failed to uncompress payload: %w`, err)
		}
		plaintext = buf
	}

	if plaintext == nil {
		return nil, fmt.Errorf(`failed to find matching recipient`)
	}

	return plaintext, nil
}

// encryptContext holds the state during JWE encryption, similar to JWS signContext
type encryptContext struct {
	calg        jwa.ContentEncryptionAlgorithm
	compression jwa.CompressionAlgorithm
	format      int
	builders    []*recipientBuilder
	protected   Headers
}

var encryptContextPool = pool.New(allocEncryptContext, freeEncryptContext)

func allocEncryptContext() *encryptContext {
	return &encryptContext{
		calg:        jwa.A256GCM(),
		compression: jwa.NoCompress(),
		format:      fmtCompact,
	}
}

func freeEncryptContext(ec *encryptContext) *encryptContext {
	ec.calg = jwa.A256GCM()
	ec.compression = jwa.NoCompress()
	ec.format = fmtCompact
	ec.builders = ec.builders[:0]
	ec.protected = nil
	return ec
}

func (ec *encryptContext) ProcessOptions(options []EncryptOption) error {
	var mergeProtected bool
	var useRawCEK bool
	for _, option := range options {
		switch option.Ident() {
		case identKey{}:
			var wk *withKey
			if err := option.Value(&wk); err != nil {
				return fmt.Errorf("jwe.encrypt: WithKey must be a *withKey: %w", err)
			}
			v, ok := wk.alg.(jwa.KeyEncryptionAlgorithm)
			if !ok {
				return fmt.Errorf("jwe.encrypt: WithKey() option must be specified using jwa.KeyEncryptionAlgorithm (got %T)", wk.alg)
			}
			if v == jwa.DIRECT() || v == jwa.ECDH_ES() {
				useRawCEK = true
			}
			ec.builders = append(ec.builders, &recipientBuilder{alg: v, key: wk.key, headers: wk.headers})
		case identContentEncryptionAlgorithm{}:
			var c jwa.ContentEncryptionAlgorithm
			if err := option.Value(&c); err != nil {
				return err
			}
			ec.calg = c
		case identCompress{}:
			var comp jwa.CompressionAlgorithm
			if err := option.Value(&comp); err != nil {
				return err
			}
			ec.compression = comp
		case identMergeProtectedHeaders{}:
			var mp bool
			if err := option.Value(&mp); err != nil {
				return err
			}
			mergeProtected = mp
		case identProtectedHeaders{}:
			var hdrs Headers
			if err := option.Value(&hdrs); err != nil {
				return err
			}
			if !mergeProtected || ec.protected == nil {
				ec.protected = hdrs
			} else {
				merged, err := ec.protected.Merge(hdrs)
				if err != nil {
					return fmt.Errorf(`failed to merge headers: %w`, err)
				}
				ec.protected = merged
			}
		case identSerialization{}:
			var fmtOpt int
			if err := option.Value(&fmtOpt); err != nil {
				return err
			}
			ec.format = fmtOpt
		}
	}

	// We need to have at least one builder
	switch l := len(ec.builders); {
	case l == 0:
		return fmt.Errorf(`missing key encryption builders: use jwe.WithKey() to specify one`)
	case l > 1:
		if ec.format == fmtCompact {
			return fmt.Errorf(`cannot use compact serialization when multiple recipients exist (check the number of WithKey() argument, or use WithJSON())`)
		}
	}

	if useRawCEK {
		if len(ec.builders) != 1 {
			return fmt.Errorf(`multiple recipients for ECDH-ES/DIRECT mode supported`)
		}
	}

	return nil
}

var msgPool = pool.New(allocMessage, freeMessage)

func allocMessage() *Message {
	return &Message{
		recipients: make([]Recipient, 0, 1),
	}
}

func freeMessage(msg *Message) *Message {
	msg.cipherText = nil
	msg.initializationVector = nil
	if hdr := msg.protectedHeaders; hdr != nil {
		headerPool.Put(hdr)
	}
	msg.protectedHeaders = nil
	msg.unprotectedHeaders = nil
	msg.recipients = nil // reuse should be done elsewhere
	msg.authenticatedData = nil
	msg.tag = nil
	msg.rawProtectedHeaders = nil
	msg.storeProtectedHeaders = false
	return msg
}

var headerPool = pool.New(NewHeaders, freeHeaders)

func freeHeaders(h Headers) Headers {
	if c, ok := h.(interface{ clear() }); ok {
		c.clear()
	}
	return h
}

var recipientPool = pool.New(NewRecipient, freeRecipient)

func freeRecipient(r Recipient) Recipient {
	if h := r.Headers(); h != nil {
		if c, ok := h.(interface{ clear() }); ok {
			c.clear()
		}
	}

	if sr, ok := r.(*stdRecipient); ok {
		sr.encryptedKey = nil
	}
	return r
}

var recipientSlicePool = pool.NewSlicePool(allocRecipientSlice, freeRecipientSlice)

func allocRecipientSlice() []Recipient {
	return make([]Recipient, 0, 1)
}

func freeRecipientSlice(rs []Recipient) []Recipient {
	for _, r := range rs {
		recipientPool.Put(r)
	}
	return rs[:0]
}

func (ec *encryptContext) EncryptMessage(payload []byte, cek []byte) ([]byte, error) {
	// Get protected headers from pool and copy contents from context
	protected := headerPool.Get()
	if userSupplied := ec.protected; userSupplied != nil {
		ec.protected = nil // Clear from context
		if err := userSupplied.Copy(protected); err != nil {
			return nil, fmt.Errorf(`failed to copy protected headers: %w`, err)
		}
	}

	// There is exactly one content encrypter.
	contentcrypt, err := content_crypt.NewGeneric(ec.calg)
	if err != nil {
		return nil, fmt.Errorf(`failed to create AES encrypter: %w`, err)
	}

	// Generate CEK if not provided
	if len(cek) <= 0 {
		bk, err := keygen.Random(contentcrypt.KeySize())
		if err != nil {
			return nil, fmt.Errorf(`failed to generate key: %w`, err)
		}
		cek = bk.Bytes()
	}

	var useRawCEK bool
	for _, builder := range ec.builders {
		if builder.alg == jwa.DIRECT() || builder.alg == jwa.ECDH_ES() {
			useRawCEK = true
			break
		}
	}

	recipients := recipientSlicePool.GetCapacity(len(ec.builders))
	defer recipientSlicePool.Put(recipients)

	for i, builder := range ec.builders {
		r := recipientPool.Get()
		defer recipientPool.Put(r)

		// some builders require hint from the contentcrypt object
		rawCEK, err := builder.Build(r, cek, ec.calg, contentcrypt)
		if err != nil {
			return nil, fmt.Errorf(`failed to create recipient #%d: %w`, i, err)
		}
		recipients = append(recipients, r)

		// Kinda feels weird, but if useRawCEK == true, we asserted earlier
		// that len(builders) == 1, so this is OK
		if useRawCEK {
			cek = rawCEK
		}
	}

	if err := protected.Set(ContentEncryptionKey, ec.calg); err != nil {
		return nil, fmt.Errorf(`failed to set "enc" in protected header: %w`, err)
	}

	if ec.compression != jwa.NoCompress() {
		payload, err = compress(payload)
		if err != nil {
			return nil, fmt.Errorf(`failed to compress payload before encryption: %w`, err)
		}
		if err := protected.Set(CompressionKey, ec.compression); err != nil {
			return nil, fmt.Errorf(`failed to set "zip" in protected header: %w`, err)
		}
	}

	// If there's only one recipient, you want to include that in the
	// protected header
	if len(recipients) == 1 {
		h, err := protected.Merge(recipients[0].Headers())
		if err != nil {
			return nil, fmt.Errorf(`failed to merge protected headers: %w`, err)
		}
		protected = h
	}

	aad, err := protected.Encode()
	if err != nil {
		return nil, fmt.Errorf(`failed to base64 encode protected headers: %w`, err)
	}

	iv, ciphertext, tag, err := contentcrypt.Encrypt(cek, payload, aad)
	if err != nil {
		return nil, fmt.Errorf(`failed to encrypt payload: %w`, err)
	}

	msg := msgPool.Get()
	defer msgPool.Put(msg)

	if err := msg.Set(CipherTextKey, ciphertext); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, CipherTextKey, err)
	}
	if err := msg.Set(InitializationVectorKey, iv); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, InitializationVectorKey, err)
	}
	if err := msg.Set(ProtectedHeadersKey, protected); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, ProtectedHeadersKey, err)
	}
	if err := msg.Set(RecipientsKey, recipients); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, RecipientsKey, err)
	}
	if err := msg.Set(TagKey, tag); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, TagKey, err)
	}

	switch ec.format {
	case fmtCompact:
		return Compact(msg)
	case fmtJSON:
		return json.Marshal(msg)
	case fmtJSONPretty:
		return json.MarshalIndent(msg, "", "  ")
	default:
		return nil, fmt.Errorf(`invalid serialization`)
	}
}

// Decrypt takes encrypted payload, and information required to decrypt the
// payload (e.g. the key encryption algorithm and the corresponding
// key to decrypt the JWE message) in its optional arguments. See
// the examples and list of options that return a DecryptOption for possible
// values. Upon successful decryptiond returns the decrypted payload.
//
// The JWE message can be either compact or full JSON format.
//
// When using `jwe.WithKeyEncryptionAlgorithm()`, you can pass a `jwa.KeyAlgorithm`
// for convenience: this is mainly to allow you to directly pass the result of `(jwk.Key).Algorithm()`.
// However, do note that while `(jwk.Key).Algorithm()` could very well contain key encryption
// algorithms, it could also contain other types of values, such as _signature algorithms_.
// In order for `jwe.Decrypt` to work properly, the `alg` parameter must be of type
// `jwa.KeyEncryptionAlgorithm` or otherwise it will cause an error.
//
// When using `jwe.WithKey()`, the value must be a private key.
// It can be either in its raw format (e.g. *rsa.PrivateKey) or a jwk.Key
//
// When the encrypted message is also compressed, the decompressed payload must be
// smaller than the size specified by the `jwe.WithMaxDecompressBufferSize` setting,
// which defaults to 10MB. If the decompressed payload is larger than this size,
// an error is returned.
//
// You can opt to change the MaxDecompressBufferSize setting globally, or on a
// per-call basis by passing the `jwe.WithMaxDecompressBufferSize` option to
// either `jwe.Settings()` or `jwe.Decrypt()`:
//
//	jwe.Settings(jwe.WithMaxDecompressBufferSize(10*1024*1024)) // changes value globally
//	jwe.Decrypt(..., jwe.WithMaxDecompressBufferSize(250*1024)) // changes just for this call
func Decrypt(buf []byte, options ...DecryptOption) ([]byte, error) {
	dc := decryptContextPool.Get()
	defer decryptContextPool.Put(dc)

	if err := dc.ProcessOptions(options); err != nil {
		return nil, decryptError{fmt.Errorf(`jwe.Decrypt: failed to process options: %w`, err)}
	}

	ret, err := dc.DecryptMessage(buf)
	if err != nil {
		return nil, decryptError{fmt.Errorf(`jwe.Decrypt: %w`, err)}
	}
	return ret, nil
}

// Parse parses the JWE message into a Message object. The JWE message
// can be either compact or full JSON format.
//
// Parse() currently does not take any options, but the API accepts it
// in anticipation of future addition.
func Parse(buf []byte, _ ...ParseOption) (*Message, error) {
	return parseJSONOrCompact(buf, false)
}

// errors are wrapped within this function, because we call it directly
// from Decrypt as well.
func parseJSONOrCompact(buf []byte, storeProtectedHeaders bool) (*Message, error) {
	buf = bytes.TrimSpace(buf)
	if len(buf) == 0 {
		return nil, parseError{fmt.Errorf(`jwe.Parse: empty buffer`)}
	}

	var msg *Message
	var err error
	if buf[0] == tokens.OpenCurlyBracket {
		msg, err = parseJSON(buf, storeProtectedHeaders)
	} else {
		msg, err = parseCompact(buf, storeProtectedHeaders)
	}

	if err != nil {
		return nil, parseError{fmt.Errorf(`jwe.Parse: %w`, err)}
	}
	return msg, nil
}

// ParseString is the same as Parse, but takes a string.
func ParseString(s string) (*Message, error) {
	msg, err := Parse([]byte(s))
	if err != nil {
		return nil, parseError{fmt.Errorf(`jwe.ParseString: %w`, err)}
	}
	return msg, nil
}

// ParseReader is the same as Parse, but takes an io.Reader.
func ParseReader(src io.Reader) (*Message, error) {
	buf, err := io.ReadAll(src)
	if err != nil {
		return nil, parseError{fmt.Errorf(`jwe.ParseReader: failed to read from io.Reader: %w`, err)}
	}
	msg, err := Parse(buf)
	if err != nil {
		return nil, parseError{fmt.Errorf(`jwe.ParseReader: %w`, err)}
	}
	return msg, nil
}

func parseJSON(buf []byte, storeProtectedHeaders bool) (*Message, error) {
	m := NewMessage()
	m.storeProtectedHeaders = storeProtectedHeaders
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil, fmt.Errorf(`failed to parse JSON: %w`, err)
	}
	return m, nil
}

func parseCompact(buf []byte, storeProtectedHeaders bool) (*Message, error) {
	var parts [5][]byte
	var ok bool

	for i := range 4 {
		parts[i], buf, ok = bytes.Cut(buf, []byte{tokens.Period})
		if !ok {
			return nil, fmt.Errorf(`compact JWE format must have five parts (%d)`, i+1)
		}
	}
	// Validate that the last part does not contain more dots
	if bytes.ContainsRune(buf, tokens.Period) {
		return nil, errors.New(`compact JWE format must have five parts, not more`)
	}
	parts[4] = buf

	hdrbuf, err := base64.Decode(parts[0])
	if err != nil {
		return nil, fmt.Errorf(`failed to parse first part of compact form: %w`, err)
	}

	protected := NewHeaders()
	if err := json.Unmarshal(hdrbuf, protected); err != nil {
		return nil, fmt.Errorf(`failed to parse header JSON: %w`, err)
	}

	ivbuf, err := base64.Decode(parts[2])
	if err != nil {
		return nil, fmt.Errorf(`failed to base64 decode iv: %w`, err)
	}

	ctbuf, err := base64.Decode(parts[3])
	if err != nil {
		return nil, fmt.Errorf(`failed to base64 decode content: %w`, err)
	}

	tagbuf, err := base64.Decode(parts[4])
	if err != nil {
		return nil, fmt.Errorf(`failed to base64 decode tag: %w`, err)
	}

	m := NewMessage()
	if err := m.Set(CipherTextKey, ctbuf); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, CipherTextKey, err)
	}
	if err := m.Set(InitializationVectorKey, ivbuf); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, InitializationVectorKey, err)
	}
	if err := m.Set(ProtectedHeadersKey, protected); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, ProtectedHeadersKey, err)
	}

	if err := m.makeDummyRecipient(string(parts[1]), protected); err != nil {
		return nil, fmt.Errorf(`failed to setup recipient: %w`, err)
	}

	if err := m.Set(TagKey, tagbuf); err != nil {
		return nil, fmt.Errorf(`failed to set %s: %w`, TagKey, err)
	}

	if storeProtectedHeaders {
		// This is later used for decryption.
		m.rawProtectedHeaders = parts[0]
	}

	return m, nil
}

type CustomDecoder = json.CustomDecoder
type CustomDecodeFunc = json.CustomDecodeFunc

// RegisterCustomField allows users to specify that a private field
// be decoded as an instance of the specified type. This option has
// a global effect.
//
// For example, suppose you have a custom field `x-birthday`, which
// you want to represent as a string formatted in RFC3339 in JSON,
// but want it back as `time.Time`.
//
// In such case you would register a custom field as follows
//
//	jws.RegisterCustomField(`x-birthday`, time.Time{})
//
// Then you can use a `time.Time` variable to extract the value
// of `x-birthday` field, instead of having to use `any`
// and later convert it to `time.Time`
//
//	var bday time.Time
//	_ = hdr.Get(`x-birthday`, &bday)
//
// If you need a more fine-tuned control over the decoding process,
// you can register a `CustomDecoder`. For example, below shows
// how to register a decoder that can parse RFC1123 format string:
//
//	jwe.RegisterCustomField(`x-birthday`, jwe.CustomDecodeFunc(func(data []byte) (any, error) {
//	  return time.Parse(time.RFC1123, string(data))
//	}))
//
// Please note that use of custom fields can be problematic if you
// are using a library that does not implement MarshalJSON/UnmarshalJSON
// and you try to roundtrip from an object to JSON, and then back to an object.
// For example, in the above example, you can _parse_ time values formatted
// in the format specified in RFC822, but when you convert an object into
// JSON, it will be formatted in RFC3339, because that's what `time.Time`
// likes to do. To avoid this, it's always better to use a custom type
// that wraps your desired type (in this case `time.Time`) and implement
// MarshalJSON and UnmashalJSON.
func RegisterCustomField(name string, object any) {
	registry.Register(name, object)
}
