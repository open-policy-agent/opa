package jwe

import (
	"context"
	"fmt"
	"sync"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

// KeyProvider is responsible for providing key(s) to encrypt or decrypt a payload.
// Multiple `jwe.KeyProvider`s can be passed to `jwe.Encrypt()` or `jwe.Decrypt()`
//
// `jwe.Encrypt()` can only accept static key providers via `jwe.WithKey()`,
// while `jwe.Decrypt()` can accept `jwe.WithKey()`, `jwe.WithKeySet()`,
// and `jwe.WithKeyProvider()`.
//
// Understanding how this works is crucial to learn how this package works.
// Here we will use `jwe.Decrypt()` as an example to show how the `KeyProvider`
// works.
//
// `jwe.Encrypt()` is straightforward: the content encryption key is encrypted
// using the provided keys, and JWS recipient objects are created for each.
//
// `jwe.Decrypt()` is a bit more involved, because there are cases you
// will want to compute/deduce/guess the keys that you would like to
// use for decryption.
//
// The first thing that `jwe.Decrypt()` needs to do is to collect the
// KeyProviders from the option list that the user provided (presented in pseudocode):
//
//	keyProviders := filterKeyProviders(options)
//
// Then, remember that a JWE message may contain multiple recipients in the
// message. For each recipient, we call on the KeyProviders to give us
// the key(s) to use on this CEK:
//
//	for r in msg.Recipients {
//	  for kp in keyProviders {
//	    kp.FetchKeys(ctx, sink, r, msg)
//	    ...
//	  }
//	}
//
// The `sink` argument passed to the KeyProvider is a temporary storage
// for the keys (either a jwk.Key or a "raw" key). The `KeyProvider`
// is responsible for sending keys into the `sink`.
//
// When called, the `KeyProvider` created by `jwe.WithKey()` sends the same key,
// `jwe.WithKeySet()` sends keys that matches a particular `kid` and `alg`,
// and finally `jwe.WithKeyProvider()` allows you to execute arbitrary
// logic to provide keys. If you are providing a custom `KeyProvider`,
// you should execute the necessary checks or retrieval of keys, and
// then send the key(s) to the sink:
//
//	sink.Key(alg, key)
//
// These keys are then retrieved and tried for each recipient, until
// a match is found:
//
//	keys := sink.Keys()
//	for key in keys {
//	  if decryptJWEKey(recipient.EncryptedKey(), key) {
//	    return OK
//	  }
//	}
type KeyProvider interface {
	FetchKeys(context.Context, KeySink, Recipient, *Message) error
}

// KeySink is a data storage where `jwe.KeyProvider` objects should
// send their keys to.
type KeySink interface {
	Key(jwa.KeyEncryptionAlgorithm, any)
}

type algKeyPair struct {
	alg jwa.KeyAlgorithm
	key any
}

type algKeySink struct {
	mu   sync.Mutex
	list []algKeyPair
}

func (s *algKeySink) Key(alg jwa.KeyEncryptionAlgorithm, key any) {
	s.mu.Lock()
	s.list = append(s.list, algKeyPair{alg, key})
	s.mu.Unlock()
}

type staticKeyProvider struct {
	alg jwa.KeyEncryptionAlgorithm
	key any
}

func (kp *staticKeyProvider) FetchKeys(_ context.Context, sink KeySink, _ Recipient, _ *Message) error {
	sink.Key(kp.alg, kp.key)
	return nil
}

type keySetProvider struct {
	set        jwk.Set
	requireKid bool
}

func (kp *keySetProvider) selectKey(sink KeySink, key jwk.Key, _ Recipient, _ *Message) error {
	if usage, ok := key.KeyUsage(); ok {
		if usage != "" && usage != jwk.ForEncryption.String() {
			return nil
		}
	}

	if v, ok := key.Algorithm(); ok {
		kalg, ok := jwa.LookupKeyEncryptionAlgorithm(v.String())
		if !ok {
			return fmt.Errorf(`invalid key encryption algorithm %s`, v)
		}

		sink.Key(kalg, key)
		return nil
	}

	return nil
}

func (kp *keySetProvider) FetchKeys(_ context.Context, sink KeySink, r Recipient, msg *Message) error {
	if kp.requireKid {
		var key jwk.Key

		wantedKid, ok := r.Headers().KeyID()
		if !ok || wantedKid == "" {
			return fmt.Errorf(`failed to find matching key: no key ID ("kid") specified in token but multiple keys available in key set`)
		}
		// Otherwise we better be able to look up the key, baby.
		v, ok := kp.set.LookupKeyID(wantedKid)
		if !ok {
			return fmt.Errorf(`failed to find key with key ID %q in key set`, wantedKid)
		}
		key = v

		return kp.selectKey(sink, key, r, msg)
	}

	for i := range kp.set.Len() {
		key, _ := kp.set.Key(i)
		if err := kp.selectKey(sink, key, r, msg); err != nil {
			continue
		}
	}
	return nil
}

// KeyProviderFunc is a type of KeyProvider that is implemented by
// a single function. You can use this to create ad-hoc `KeyProvider`
// instances.
type KeyProviderFunc func(context.Context, KeySink, Recipient, *Message) error

func (kp KeyProviderFunc) FetchKeys(ctx context.Context, sink KeySink, r Recipient, msg *Message) error {
	return kp(ctx, sink, r, msg)
}
