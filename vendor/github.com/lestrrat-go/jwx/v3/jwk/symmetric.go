package jwk

import (
	"crypto"
	"fmt"
	"reflect"

	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/jwa"
)

func init() {
	RegisterKeyExporter(jwa.OctetSeq(), KeyExportFunc(octetSeqToRaw))
}

func (k *symmetricKey) Import(rawKey []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(rawKey) == 0 {
		return fmt.Errorf(`non-empty []byte key required`)
	}

	k.octets = rawKey

	return nil
}

var symmetricConvertibleKeys = []reflect.Type{
	reflect.TypeOf((*SymmetricKey)(nil)).Elem(),
}

func octetSeqToRaw(key Key, hint any) (any, error) {
	extracted, err := extractEmbeddedKey(key, symmetricConvertibleKeys)
	if err != nil {
		return nil, fmt.Errorf(`failed to extract embedded key: %w`, err)
	}

	switch key := extracted.(type) {
	case SymmetricKey:
		switch hint.(type) {
		case *[]byte, *any:
		default:
			return nil, fmt.Errorf(`invalid destination object type %T for symmetric key: %w`, hint, ContinueError())
		}

		locker, ok := key.(rlocker)
		if ok {
			locker.rlock()
			defer locker.runlock()
		}

		ooctets, ok := key.Octets()
		if !ok {
			return nil, fmt.Errorf(`jwk.SymmetricKey: missing "k" field`)
		}

		octets := make([]byte, len(ooctets))
		copy(octets, ooctets)
		return octets, nil
	default:
		return nil, ContinueError()
	}
}

// Thumbprint returns the JWK thumbprint using the indicated
// hashing algorithm, according to RFC 7638
func (k *symmetricKey) Thumbprint(hash crypto.Hash) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	var octets []byte
	if err := Export(k, &octets); err != nil {
		return nil, fmt.Errorf(`failed to export symmetric key: %w`, err)
	}

	h := hash.New()
	fmt.Fprint(h, `{"k":"`)
	fmt.Fprint(h, base64.EncodeToString(octets))
	fmt.Fprint(h, `","kty":"oct"}`)
	return h.Sum(nil), nil
}

func (k *symmetricKey) PublicKey() (Key, error) {
	newKey := newSymmetricKey()

	for _, key := range k.Keys() {
		var v any
		if err := k.Get(key, &v); err != nil {
			return nil, fmt.Errorf(`failed to get field %q: %w`, key, err)
		}

		if err := newKey.Set(key, v); err != nil {
			return nil, fmt.Errorf(`failed to set field %q: %w`, key, err)
		}
	}
	return newKey, nil
}

func (k *symmetricKey) Validate() error {
	octets, ok := k.Octets()
	if !ok || len(octets) == 0 {
		return NewKeyValidationError(fmt.Errorf(`jwk.SymmetricKey: missing "k" field`))
	}
	return nil
}
