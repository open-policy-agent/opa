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

// SaveOptions is a list of options which can be set when writing bundle data
// to disk. Currently, only setting of the bundle's ETag is supported.
type SaveOptions struct {
	Etag string
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
func LoadBundleFromDisk(path, name string, bvc *bundle.VerificationConfig) (*bundle.Bundle, error) {
	bundlePath := filepath.Join(path, name, "bundle.tar.gz")
	bundlePackagePath := filepath.Join(path, name, "bundlePackage.tar.gz")

	// if a bundlePackage exists, use that
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
		if bvc != nil {
			r = r.WithBundleVerificationConfig(bvc)
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
	if _, err := os.Stat(bundlePath); err == nil {
		f, err := os.Open(filepath.Join(bundlePath))
		if err != nil {
			return nil, err
		}
		defer f.Close()

		r := bundle.NewCustomReader(bundle.NewTarballLoaderWithBaseURL(f, ""))

		if bvc != nil {
			r = r.WithBundleVerificationConfig(bvc)
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

// SaveBundleToDisk saves the given raw bytes representing the bundle's content to disk. Passing nil for the rawManifest
// will result in no manifest being persisted to disk.
func SaveBundleToDisk(path string, rawBundle io.Reader, rawManifest io.Reader) (string, string, error) {
	var cleanupOperations []func()
	var failed bool
	defer func() {
		if failed {
			for _, cleanup := range cleanupOperations {
				cleanup()
			}
		}
	}()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "", "", err
		}
	}

	// supplying no bundle data is an error case
	if rawBundle == nil {
		return "", "", fmt.Errorf("no raw bundle bytes to persist to disk")
	}

	// create a temporary file to write the bundle to
	destBundle, err := os.CreateTemp(path, ".bundle.tar.gz.*.tmp")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temporary bundle file: %w", err)
	}
	defer destBundle.Close()

	cleanupOperations = append(cleanupOperations, func() {
		os.Remove(destBundle.Name())
	})

	// handle the optional case where a bundle manifest is provided
	var errManifest error
	var destManifestName string
	if rawManifest != nil {
		destManifest, err := os.CreateTemp(path, ".manifest.json.*.tmp")
		if err != nil {
			return "", "", err
		}
		defer destManifest.Close()
		cleanupOperations = append(cleanupOperations, func() {
			os.Remove(destManifest.Name())
		})

		destManifestName = destManifest.Name()

		// write the manifest to disk first
		_, errManifest = io.Copy(destManifest, rawManifest)
	}

	// write the bundle to disk
	_, errBundle := io.Copy(destBundle, rawBundle)

	// handle errors from both the bundle or manifest write operations
	if errBundle != nil && errManifest != nil {
		failed = true
		return "", "", fmt.Errorf("failed to save bundle and manifest to disk: %s, %s", errBundle, errManifest)
	}
	if errBundle != nil {
		failed = true
		return "", "", fmt.Errorf("failed to save bundle to disk: %s", errBundle)
	}
	if errManifest != nil {
		failed = true
		return "", "", fmt.Errorf("failed to save manifest to disk: %s", errManifest)
	}

	return destBundle.Name(), destManifestName, nil
}

func SaveBundlePackageToDisk(path string, rawBundle io.Reader, opts *SaveOptions) error {
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
	zw.Name = "bundlePackage.tar.gz"

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
		filepath.Join(path, "bundlePackage.tar.gz"),
	)
}
