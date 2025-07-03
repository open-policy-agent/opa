package jws

import (
	"fmt"
	"sync"

	"github.com/lestrrat-go/jwx/v3/jwa"
)

// Signer2 is an interface that represents a per-signature algorithm signing
// operation.
type Signer2 interface {
	Algorithm() jwa.SignatureAlgorithm

	// Sign takes a key and a payload, and returns the signature for the payload.
	// The key type is restricted by the signature algorithm that this
	// signer is associated with.
	//
	// (Note to users of legacy Signer interface: the method signature
	// is different from the legacy Signer interface)
	Sign(key any, payload []byte) ([]byte, error)
}

var muSigner2DB sync.RWMutex
var signer2DB = make(map[jwa.SignatureAlgorithm]Signer2)

type SignerFactory interface {
	Create() (Signer, error)
}
type SignerFactoryFn func() (Signer, error)

func (fn SignerFactoryFn) Create() (Signer, error) {
	return fn()
}

// SignerFor returns a Signer2 for the given signature algorithm.
//
// Currently, this function will never fail. It will always return a
// valid Signer2 object. The heuristic is as follows:
//  1. If a Signer2 is registered for the given algorithm, it will return that.
//  2. If a legacy Signer(Factory) is registered for the given algorithm, it will
//     return a Signer2 that wraps the legacy Signer.
//  3. If no Signer2 or legacy Signer(Factory) is registered, it will return a
//     default signer that uses jwsbb.Sign.
//
// jwsbb.Sign knows how to handle a static set of algorithms, so if the
// algorithm is not supported, it will return an error when you call
// `Sign` on the default signer.
func SignerFor(alg jwa.SignatureAlgorithm) (Signer2, error) {
	muSigner2DB.RLock()
	defer muSigner2DB.RUnlock()

	signer, ok := signer2DB[alg]
	if ok {
		return signer, nil
	}

	s1, err := legacySignerFor(alg)
	if err == nil {
		return signerAdapter{signer: s1}, nil
	}

	return defaultSigner{alg: alg}, nil
}

var muSignerDB sync.RWMutex
var signerDB = make(map[jwa.SignatureAlgorithm]SignerFactory)

// RegisterSigner is used to register a signer for the given
// algorithm.
//
// Please note that this function is intended to be passed a
// signer object as its second argument, but due to historical
// reasons the function signature is defined as taking `any` type.
//
// You should create a signer object that implements the `Signer2`
// interface to register a signer, unless you have legacy code that
// plugged into the `SignerFactory` interface.
//
// Unlike the `UnregisterSigner` function, this function automatically
// calls `jwa.RegisterSignatureAlgorithm` to register the algorithm
// in this module's algorithm database.
func RegisterSigner(alg jwa.SignatureAlgorithm, f any) error {
	jwa.RegisterSignatureAlgorithm(alg)
	switch s := f.(type) {
	case Signer2:
		muSigner2DB.Lock()
		signer2DB[alg] = s
		muSigner2DB.Unlock()

		// delete the other signer, if there was one
		muSignerDB.Lock()
		delete(signerDB, alg)
		muSignerDB.Unlock()
	case SignerFactory:
		muSignerDB.Lock()
		signerDB[alg] = s
		muSignerDB.Unlock()

		// Remove previous signer, if there was one
		removeSigner(alg)

		muSigner2DB.Lock()
		delete(signer2DB, alg)
		muSigner2DB.Unlock()
	default:
		return fmt.Errorf(`jws.RegisterSigner: unsupported type %T for algorithm %q`, f, alg)
	}
	return nil
}

// UnregisterSigner removes the signer factory associated with
// the given algorithm, as well as the signer instance created
// by the factory.
//
// Note that when you call this function, the algorithm itself is
// not automatically unregistered from this module's algorithm database.
// This is because the algorithm may still be required for verification or
// some other operation (however unlikely, it is still possible).
// Therefore, in order to completely remove the algorithm, you must
// call `jwa.UnregisterSignatureAlgorithm` yourself.
func UnregisterSigner(alg jwa.SignatureAlgorithm) {
	muSigner2DB.Lock()
	delete(signer2DB, alg)
	muSigner2DB.Unlock()

	muSignerDB.Lock()
	delete(signerDB, alg)
	muSignerDB.Unlock()
	// Remove previous signer
	removeSigner(alg)
}

// NewSigner creates a signer that signs payloads using the given signature algorithm.
// This function is deprecated. You should use `SignerFor()` instead.
//
// This function only exists for backwards compatibility, but will not work
// unless you enable the legacy support mode by calling jws.Settings(jws.WithLegacySigners(true)).
func NewSigner(alg jwa.SignatureAlgorithm) (Signer, error) {
	muSignerDB.RLock()
	f, ok := signerDB[alg]
	muSignerDB.RUnlock()

	if ok {
		return f.Create()
	}
	return nil, fmt.Errorf(`jws.NewSigner: unsupported signature algorithm "%s"`, alg)
}

type noneSigner struct{}

func (noneSigner) Algorithm() jwa.SignatureAlgorithm {
	return jwa.NoSignature()
}

func (noneSigner) Sign([]byte, any) ([]byte, error) {
	return nil, nil
}
