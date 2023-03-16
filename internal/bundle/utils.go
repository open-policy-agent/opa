// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/resolver/wasm"
	"github.com/open-policy-agent/opa/storage"
)

// BundlePackageFileName is the name of the file used to store bundle and associated metadata
const BundlePackageFileName = "bundlePackage.tar.gz"

// SaveOptions is a list of options which can be set when writing bundle data
// to disk. Currently, only setting of the bundle's ETag is supported.
type SaveOptions struct {
	Etag string
}

// LoadOptions is a list of options which can be set when loading a bundle from disk.
// Currently, only setting VerificationConfig is supported.
type LoadOptions struct {
	VerificationConfig *bundle.VerificationConfig
}

// bundlePackage represents a bundle and associated metadata which is ready to be
// serialized to disk.
type bundlePackage struct {
	Etag   string `json:"etag"`
	Bundle []byte `json:"bundle"`
}

// LoadWasmResolversFromStore will lookup all Wasm modules from the store along with the
// associated bundle manifest configuration and instantiate the respective resolvers.
func LoadWasmResolversFromStore(ctx context.Context, store storage.Store, txn storage.Transaction, otherBundles map[string]*bundle.Bundle) ([]*wasm.Resolver, error) {
	bundleNames, err := bundle.ReadBundleNamesFromStore(ctx, store, txn)
	if err != nil && !storage.IsNotFound(err) {
		return nil, err
	}

	var resolversToLoad []*bundle.WasmModuleFile
	for _, bundleName := range bundleNames {
		var wasmResolverConfigs []bundle.WasmResolver
		rawModules := map[string][]byte{}

		// Save round-tripping the bundle that was just activated
		if _, ok := otherBundles[bundleName]; ok {
			wasmResolverConfigs = otherBundles[bundleName].Manifest.WasmResolvers
			for _, wmf := range otherBundles[bundleName].WasmModules {
				rawModules[wmf.Path] = wmf.Raw
			}
		} else {
			wasmResolverConfigs, err = bundle.ReadWasmMetadataFromStore(ctx, store, txn, bundleName)
			if err != nil && !storage.IsNotFound(err) {
				return nil, fmt.Errorf("failed to read wasm module manifest from store: %s", err)
			}
			rawModules, err = bundle.ReadWasmModulesFromStore(ctx, store, txn, bundleName)
			if err != nil && !storage.IsNotFound(err) {
				return nil, fmt.Errorf("failed to read wasm modules from store: %s", err)
			}
		}

		for path, raw := range rawModules {
			wmf := &bundle.WasmModuleFile{
				URL:  path,
				Path: path,
				Raw:  raw,
			}
			for _, resolverConf := range wasmResolverConfigs {
				if resolverConf.Module == path {
					ref, err := ast.PtrRef(ast.DefaultRootDocument, resolverConf.Entrypoint)
					if err != nil {
						return nil, fmt.Errorf("failed to parse wasm module entrypoint '%s': %s", resolverConf.Entrypoint, err)
					}
					wmf.Entrypoints = append(wmf.Entrypoints, ref)
				}
			}
			if len(wmf.Entrypoints) > 0 {
				resolversToLoad = append(resolversToLoad, wmf)
			}
		}
	}

	var resolvers []*wasm.Resolver
	if len(resolversToLoad) > 0 {
		// Get a full snapshot of the current data (including any from "outside" the bundles)
		data, err := store.Read(ctx, txn, storage.Path{})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize wasm runtime: %s", err)
		}

		for _, wmf := range resolversToLoad {
			resolver, err := wasm.New(wmf.Entrypoints, wmf.Raw, data)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize wasm module for entrypoints '%s': %s", wmf.Entrypoints, err)
			}
			resolvers = append(resolvers, resolver)
		}
	}
	return resolvers, nil
}

// LoadBundleFromDisk loads a previously persisted activated bundle from disk
func LoadBundleFromDisk(path string, opts *LoadOptions) (*bundle.Bundle, error) {
	// if a bundlePackage exists, use that
	bundlePackagePath := filepath.Join(path, BundlePackageFileName)
	if _, err := os.Stat(bundlePackagePath); err == nil {
		f, err := os.Open(filepath.Join(bundlePackagePath))
		if err != nil {
			return nil, err
		}

		zr, err := gzip.NewReader(f)
		if err != nil {
			log.Fatal(err)
		}

		var bundlePackage bundlePackage
		err = json.NewDecoder(zr).Decode(&bundlePackage)
		if err != nil {
			return nil, err
		}

		r := bundle.NewReader(bytes.NewReader(bundlePackage.Bundle))
		if opts.VerificationConfig != nil {
			r = r.WithBundleVerificationConfig(opts.VerificationConfig)
		}
		if bundlePackage.Etag != "" {
			r = r.WithBundleEtag(bundlePackage.Etag)
		}

		b, err := r.Read()
		if err != nil {
			return nil, err
		}

		return &b, nil
	}

	// otherwise, load a legacy bundle file from disk. This does now support
	// setting of the bundle etag.
	bundlePath := filepath.Join(path, "bundle.tar.gz")
	if _, err := os.Stat(bundlePath); err == nil {
		f, err := os.Open(filepath.Join(bundlePath))
		if err != nil {
			return nil, err
		}
		defer f.Close()

		r := bundle.NewCustomReader(bundle.NewTarballLoaderWithBaseURL(f, ""))

		if opts.VerificationConfig != nil {
			r = r.WithBundleVerificationConfig(opts.VerificationConfig)
		}

		b, err := r.Read()
		if err != nil {
			return nil, err
		}

		return &b, nil
	} else if os.IsNotExist(err) {
		return nil, nil
	} else {
		return nil, err
	}
}

// SaveBundleToDisk persists a bundle to disk. Bundles are wrapped in a 'bundlePackage' in order
// to support additional metadata (e.g. etag) that is not part of the original bundle.
func SaveBundleToDisk(path string, rawBundle io.Reader, opts *SaveOptions) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	// supplying no bundle data is an error case
	if rawBundle == nil {
		return fmt.Errorf("no raw bundle bytes to persist to disk")
	}

	// create a temporary file to write the bundlepackage to
	destBundlePackage, err := os.CreateTemp(path, ".bundlePackage.tar.gz.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary bundle package file: %w", err)
	}
	defer destBundlePackage.Close()

	rawBundleBytes, err := io.ReadAll(rawBundle)
	if err != nil {
		return fmt.Errorf("failed to read raw bundle bytes: %w", err)
	}

	bp := bundlePackage{
		Bundle: rawBundleBytes,
		Etag:   opts.Etag,
	}

	jsonPackageData, err := json.Marshal(bp)
	if err != nil {
		return fmt.Errorf("failed to marshal bundle package data: %w", err)
	}

	zw := gzip.NewWriter(destBundlePackage)
	// this should be the eventual target name of the bundle package file
	zw.Name = BundlePackageFileName

	_, err = zw.Write(jsonPackageData)
	if err != nil {
		return fmt.Errorf("failed to write bundle package data to gzip writer: %w", err)
	}

	err = zw.Close()
	if err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return os.Rename(
		destBundlePackage.Name(),
		filepath.Join(path, BundlePackageFileName),
	)
}
