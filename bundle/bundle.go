// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle implements bundle loading.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
// # Package bundle provide helpers that assist in creating the verification and signing key configuration
//
// # Package bundle provide helpers that assist in the creating a signed bundle
//
// Package bundle provide helpers that assist in the bundle signature verification process
package bundle

import (
	"context"
	"io"
	"io/fs"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	v1 "github.com/open-policy-agent/opa/v1/bundle"
)

// Common file extensions and file names.
const (
	RegoExt        = v1.RegoExt
	WasmFile       = v1.WasmFile
	PlanFile       = v1.PlanFile
	ManifestExt    = v1.ManifestExt
	SignaturesFile = v1.SignaturesFile

	DefaultSizeLimitBytes = v1.DefaultSizeLimitBytes
	DeltaBundleType       = v1.DeltaBundleType
	SnapshotBundleType    = v1.SnapshotBundleType
)

// Bundle represents a loaded bundle. The bundle can contain data and policies.
type Bundle = v1.Bundle

// Raw contains raw bytes representing the bundle's content
type Raw = v1.Raw

// Patch contains an array of objects wherein each object represents the patch operation to be
// applied to the bundle data.
type Patch = v1.Patch

// PatchOperation models a single patch operation against a document.
type PatchOperation = v1.PatchOperation

// SignaturesConfig represents an array of JWTs that encapsulate the signatures for the bundle.
type SignaturesConfig = v1.SignaturesConfig

// DecodedSignature represents the decoded JWT payload.
type DecodedSignature = v1.DecodedSignature

// FileInfo contains the hashing algorithm used, resulting digest etc.
type FileInfo = v1.FileInfo

// NewFile returns a new FileInfo.
func NewFile(name, hash, alg string) FileInfo {
	return v1.NewFile(name, hash, alg)
}

// Manifest represents the manifest from a bundle. The manifest may contain
// metadata such as the bundle revision.
type Manifest = v1.Manifest

// WasmResolver maps a wasm module to an entrypoint ref.
type WasmResolver = v1.WasmResolver

// ModuleFile represents a single module contained in a bundle.
type ModuleFile = v1.ModuleFile

// WasmModuleFile represents a single wasm module contained in a bundle.
type WasmModuleFile = v1.WasmModuleFile

// PlanModuleFile represents a single plan module contained in a bundle.
//
// NOTE(tsandall): currently the plans are just opaque binary blobs. In the
// future we could inject the entrypoints so that the plans could be executed
// inside of OPA proper like we do for Wasm modules.
type PlanModuleFile = v1.PlanModuleFile

// Reader contains the reader to load the bundle from.
type Reader = v1.Reader

// NewReader is deprecated. Use NewCustomReader instead.
func NewReader(r io.Reader) *Reader {
	return v1.NewReader(r).WithRegoVersion(ast.DefaultRegoVersion)
}

// NewCustomReader returns a new Reader configured to use the
// specified DirectoryLoader.
func NewCustomReader(loader DirectoryLoader) *Reader {
	return v1.NewCustomReader(loader).WithRegoVersion(ast.DefaultRegoVersion)
}

// Write is deprecated. Use NewWriter instead.
func Write(w io.Writer, bundle Bundle) error {
	return v1.Write(w, bundle)
}

// Writer implements bundle serialization.
type Writer = v1.Writer

// NewWriter returns a bundle writer that writes to w.
func NewWriter(w io.Writer) *Writer {
	return v1.NewWriter(w)
}

// Merge accepts a set of bundles and merges them into a single result bundle. If there are
// any conflicts during the merge (e.g., with roots) an error is returned. The result bundle
// will have an empty revision except in the special case where a single bundle is provided
// (and in that case the bundle is just returned unmodified.)
func Merge(bundles []*Bundle) (*Bundle, error) {
	return MergeWithRegoVersion(bundles, ast.DefaultRegoVersion, false)
}

// MergeWithRegoVersion creates a merged bundle from the provided bundles, similar to Merge.
// If more than one bundle is provided, the rego version of the result bundle is set to the provided regoVersion.
// Any Rego files in a bundle of conflicting rego version will be marked in the result's manifest with the rego version
// of its original bundle. If the Rego file already had an overriding rego version, it will be preserved.
// If a single bundle is provided, it will retain any rego version information it already had. If it has none, the
// provided regoVersion will be applied to it.
// If usePath is true, per-file rego-versions will be calculated using the file's ModuleFile.Path; otherwise, the file's
// ModuleFile.URL will be used.
func MergeWithRegoVersion(bundles []*Bundle, regoVersion ast.RegoVersion, usePath bool) (*Bundle, error) {
	if regoVersion == ast.RegoUndefined {
		regoVersion = ast.DefaultRegoVersion
	}

	return v1.MergeWithRegoVersion(bundles, regoVersion, usePath)
}

// RootPathsOverlap takes in two bundle root paths and returns true if they overlap.
func RootPathsOverlap(pathA string, pathB string) bool {
	return v1.RootPathsOverlap(pathA, pathB)
}

// RootPathsContain takes a set of bundle root paths and returns true if the path is contained.
func RootPathsContain(roots []string, path string) bool {
	return v1.RootPathsContain(roots, path)
}

// Descriptor contains information about a file and
// can be used to read the file contents.
type Descriptor = v1.Descriptor

func NewDescriptor(url, path string, reader io.Reader) *Descriptor {
	return v1.NewDescriptor(url, path, reader)
}

type PathFormat = v1.PathFormat

const (
	Chrooted    = v1.Chrooted
	SlashRooted = v1.SlashRooted
	Passthrough = v1.Passthrough
)

// DirectoryLoader defines an interface which can be used to load
// files from a directory by iterating over each one in the tree.
type DirectoryLoader = v1.DirectoryLoader

// NewDirectoryLoader returns a basic DirectoryLoader implementation
// that will load files from a given root directory path.
func NewDirectoryLoader(root string) DirectoryLoader {
	return v1.NewDirectoryLoader(root)
}

// NewTarballLoader is deprecated. Use NewTarballLoaderWithBaseURL instead.
func NewTarballLoader(r io.Reader) DirectoryLoader {
	return v1.NewTarballLoader(r)
}

// NewTarballLoaderWithBaseURL returns a new DirectoryLoader that reads
// files out of a gzipped tar archive. The file URLs will be prefixed
// with the baseURL.
func NewTarballLoaderWithBaseURL(r io.Reader, baseURL string) DirectoryLoader {
	return v1.NewTarballLoaderWithBaseURL(r, baseURL)
}

func NewIterator(raw []Raw) storage.Iterator {
	return v1.NewIterator(raw)
}

// NewFSLoader returns a basic DirectoryLoader implementation
// that will load files from a fs.FS interface
func NewFSLoader(filesystem fs.FS) (DirectoryLoader, error) {
	return v1.NewFSLoader(filesystem)
}

// NewFSLoaderWithRoot returns a basic DirectoryLoader implementation
// that will load files from a fs.FS interface at the supplied root
func NewFSLoaderWithRoot(filesystem fs.FS, root string) DirectoryLoader {
	return v1.NewFSLoaderWithRoot(filesystem, root)
}

// HashingAlgorithm represents a subset of hashing algorithms implemented in Go
type HashingAlgorithm = v1.HashingAlgorithm

// Supported values for HashingAlgorithm
const (
	MD5       = v1.MD5
	SHA1      = v1.SHA1
	SHA224    = v1.SHA224
	SHA256    = v1.SHA256
	SHA384    = v1.SHA384
	SHA512    = v1.SHA512
	SHA512224 = v1.SHA512224
	SHA512256 = v1.SHA512256
)

// SignatureHasher computes a signature digest for a file with (structured or unstructured) data and policy
type SignatureHasher = v1.SignatureHasher

// NewSignatureHasher returns a signature hasher suitable for a particular hashing algorithm
func NewSignatureHasher(alg HashingAlgorithm) (SignatureHasher, error) {
	return v1.NewSignatureHasher(alg)
}

// KeyConfig holds the keys used to sign or verify bundles and tokens
// Moved to own package, alias kept for backwards compatibility
type KeyConfig = v1.KeyConfig

// VerificationConfig represents the key configuration used to verify a signed bundle
type VerificationConfig = v1.VerificationConfig

// NewVerificationConfig return a new VerificationConfig
func NewVerificationConfig(keys map[string]*KeyConfig, id, scope string, exclude []string) *VerificationConfig {
	return v1.NewVerificationConfig(keys, id, scope, exclude)
}

// SigningConfig represents the key configuration used to generate a signed bundle
type SigningConfig = v1.SigningConfig

// NewSigningConfig return a new SigningConfig
func NewSigningConfig(key, alg, claimsPath string) *SigningConfig {
	return v1.NewSigningConfig(key, alg, claimsPath)
}

// Signer is the interface expected for implementations that generate bundle signatures.
type Signer v1.Signer

// GenerateSignedToken will retrieve the Signer implementation based on the Plugin specified
// in SigningConfig, and call its implementation of GenerateSignedToken. The signer generates
// a signed token given the list of files to be included in the payload and the bundle
// signing config. The keyID if non-empty, represents the value for the "keyid" claim in the token.
func GenerateSignedToken(files []FileInfo, sc *SigningConfig, keyID string) (string, error) {
	return v1.GenerateSignedToken(files, sc, keyID)
}

// DefaultSigner is the default bundle signing implementation. It signs bundles by generating
// a JWT and signing it using a locally-accessible private key.
type DefaultSigner v1.DefaultSigner

// GetSigner returns the Signer registered under the given id
func GetSigner(id string) (Signer, error) {
	return v1.GetSigner(id)
}

// RegisterSigner registers a Signer under the given id
func RegisterSigner(id string, s Signer) error {
	return v1.RegisterSigner(id, s)
}

// BundlesBasePath is the storage path used for storing bundle metadata
var BundlesBasePath = v1.BundlesBasePath

// Note: As needed these helpers could be memoized.

// ManifestStoragePath is the storage path used for the given named bundle manifest.
func ManifestStoragePath(name string) storage.Path {
	return v1.ManifestStoragePath(name)
}

// EtagStoragePath is the storage path used for the given named bundle etag.
func EtagStoragePath(name string) storage.Path {
	return v1.EtagStoragePath(name)
}

// ReadBundleNamesFromStore will return a list of bundle names which have had their metadata stored.
func ReadBundleNamesFromStore(ctx context.Context, store storage.Store, txn storage.Transaction) ([]string, error) {
	return v1.ReadBundleNamesFromStore(ctx, store, txn)
}

// WriteManifestToStore will write the manifest into the storage. This function is called when
// the bundle is activated.
func WriteManifestToStore(ctx context.Context, store storage.Store, txn storage.Transaction, name string, manifest Manifest) error {
	return v1.WriteManifestToStore(ctx, store, txn, name, manifest)
}

// WriteEtagToStore will write the bundle etag into the storage. This function is called when the bundle is activated.
func WriteEtagToStore(ctx context.Context, store storage.Store, txn storage.Transaction, name, etag string) error {
	return v1.WriteEtagToStore(ctx, store, txn, name, etag)
}

// EraseManifestFromStore will remove the manifest from storage. This function is called
// when the bundle is deactivated.
func EraseManifestFromStore(ctx context.Context, store storage.Store, txn storage.Transaction, name string) error {
	return v1.EraseManifestFromStore(ctx, store, txn, name)
}

// ReadWasmModulesFromStore will write Wasm module resolver metadata from the store.
func ReadWasmModulesFromStore(ctx context.Context, store storage.Store, txn storage.Transaction, name string) (map[string][]byte, error) {
	return v1.ReadWasmModulesFromStore(ctx, store, txn, name)
}

// ReadBundleRootsFromStore returns the roots in the specified bundle.
// If the bundle is not activated, this function will return
// storage NotFound error.
func ReadBundleRootsFromStore(ctx context.Context, store storage.Store, txn storage.Transaction, name string) ([]string, error) {
	return v1.ReadBundleRootsFromStore(ctx, store, txn, name)
}

// ReadBundleRevisionFromStore returns the revision in the specified bundle.
// If the bundle is not activated, this function will return
// storage NotFound error.
func ReadBundleRevisionFromStore(ctx context.Context, store storage.Store, txn storage.Transaction, name string) (string, error) {
	return v1.ReadBundleRevisionFromStore(ctx, store, txn, name)
}

// ReadBundleMetadataFromStore returns the metadata in the specified bundle.
// If the bundle is not activated, this function will return
// storage NotFound error.
func ReadBundleMetadataFromStore(ctx context.Context, store storage.Store, txn storage.Transaction, name string) (map[string]any, error) {
	return v1.ReadBundleMetadataFromStore(ctx, store, txn, name)
}

// ReadBundleEtagFromStore returns the etag for the specified bundle.
// If the bundle is not activated, this function will return
// storage NotFound error.
func ReadBundleEtagFromStore(ctx context.Context, store storage.Store, txn storage.Transaction, name string) (string, error) {
	return v1.ReadBundleEtagFromStore(ctx, store, txn, name)
}

// ActivateOpts defines options for the Activate API call.
type ActivateOpts = v1.ActivateOpts

// Activate the bundle(s) by loading into the given Store. This will load policies, data, and record
// the manifest in storage. The compiler provided will have had the polices compiled on it.
func Activate(opts *ActivateOpts) error {
	return v1.Activate(setActivateDefaultRegoVersion(opts))
}

// DeactivateOpts defines options for the Deactivate API call
type DeactivateOpts = v1.DeactivateOpts

// Deactivate the bundle(s). This will erase associated data, policies, and the manifest entry from the store.
func Deactivate(opts *DeactivateOpts) error {
	return v1.Deactivate(setDeactivateDefaultRegoVersion(opts))
}

// LegacyWriteManifestToStore will write the bundle manifest to the older single (unnamed) bundle manifest location.
//
// Deprecated: Use WriteManifestToStore and named bundles instead.
func LegacyWriteManifestToStore(ctx context.Context, store storage.Store, txn storage.Transaction, manifest Manifest) error {
	return v1.LegacyWriteManifestToStore(ctx, store, txn, manifest)
}

// LegacyEraseManifestFromStore will erase the bundle manifest from the older single (unnamed) bundle manifest location.
//
// Deprecated: Use WriteManifestToStore and named bundles instead.
func LegacyEraseManifestFromStore(ctx context.Context, store storage.Store, txn storage.Transaction) error {
	return v1.LegacyEraseManifestFromStore(ctx, store, txn)
}

// LegacyReadRevisionFromStore will read the bundle manifest revision from the older single (unnamed) bundle manifest location.
//
// Deprecated: Use ReadBundleRevisionFromStore and named bundles instead.
func LegacyReadRevisionFromStore(ctx context.Context, store storage.Store, txn storage.Transaction) (string, error) {
	return v1.LegacyReadRevisionFromStore(ctx, store, txn)
}

// ActivateLegacy calls Activate for the bundles but will also write their manifest to the older unnamed store location.
//
// Deprecated: Use Activate with named bundles instead.
func ActivateLegacy(opts *ActivateOpts) error {
	return v1.ActivateLegacy(opts)
}

func setActivateDefaultRegoVersion(opts *ActivateOpts) *ActivateOpts {
	if opts == nil {
		return nil
	}

	if opts.ParserOptions.RegoVersion == ast.RegoUndefined {
		cpy := *opts
		cpy.ParserOptions.RegoVersion = ast.DefaultRegoVersion
		return &cpy
	}

	return opts
}

func setDeactivateDefaultRegoVersion(opts *DeactivateOpts) *DeactivateOpts {
	if opts == nil {
		return nil
	}

	if opts.ParserOptions.RegoVersion == ast.RegoUndefined {
		cpy := *opts
		cpy.ParserOptions.RegoVersion = ast.DefaultRegoVersion
		return &cpy
	}

	return opts
}

// Verifier is the interface expected for implementations that verify bundle signatures.
type Verifier v1.Verifier

// VerifyBundleSignature will retrieve the Verifier implementation based
// on the Plugin specified in SignaturesConfig, and call its implementation
// of VerifyBundleSignature. VerifyBundleSignature verifies the bundle signature
// using the given public keys or secret. If a signature is verified, it keeps
// track of the files specified in the JWT payload
func VerifyBundleSignature(sc SignaturesConfig, bvc *VerificationConfig) (map[string]FileInfo, error) {
	return v1.VerifyBundleSignature(sc, bvc)
}

// DefaultVerifier is the default bundle verification implementation. It verifies bundles by checking
// the JWT signature using a locally-accessible public key.
type DefaultVerifier = v1.DefaultVerifier

// GetVerifier returns the Verifier registered under the given id
func GetVerifier(id string) (Verifier, error) {
	return v1.GetVerifier(id)
}

// RegisterVerifier registers a Verifier under the given id
func RegisterVerifier(id string, v Verifier) error {
	return v1.RegisterVerifier(id, v)
}
