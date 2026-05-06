// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inspect

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/util"
	"sigs.k8s.io/yaml"
)

// Info represents information about a bundle.
type Info struct {
	Manifest    *bundle.Manifest        `json:"manifest,omitempty"`
	Signatures  bundle.SignaturesConfig `json:"signatures_config"`
	WasmModules []map[string]any        `json:"wasm_modules,omitempty"`
	Namespaces  map[string][]string     `json:"namespaces,omitempty"`
	Annotations []*ast.AnnotationsRef   `json:"annotations,omitempty"`
	Required    *ast.Capabilities       `json:"capabilities,omitempty"`
}

func File(path string, includeAnnotations bool) (*Info, error) {
	return FileForRegoVersion(ast.RegoV0, path, includeAnnotations)
}

func FileForRegoVersion(regoVersion ast.RegoVersion, path string, includeAnnotations bool) (*Info, error) {
	if strings.HasSuffix(path, bundle.RegoExt) {
		return fileInfoForRegoVersion(regoVersion, path, includeAnnotations)
	}

	return bundleOrDirInfoForRegoVersion(regoVersion, path, includeAnnotations)
}

// DataFileInfo returns an Info struct describing the given data files.
// It accepts JSON (.json) and YAML (.yaml, .yml) files.
func DataFileInfo(paths []string) (*Info, error) {
	bi := &Info{
		Namespaces: make(map[string][]string),
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing path %s: %w", path, err)
		}

		if info.IsDir() {
			return nil, fmt.Errorf("path %s is a directory, use positional argument for directories", path)
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			return nil, fmt.Errorf("file %s is not a JSON or YAML data file", path)
		}

		// Read the file to validate it can be parsed
		bs, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", path, err)
		}

		if ext == ".yaml" || ext == ".yml" {
			if _, err := yaml.YAMLToJSON(bs); err != nil {
				return nil, fmt.Errorf("error parsing YAML file %s: %w", path, err)
			}
		} else {
			var x any
			if err := util.UnmarshalJSON(bs, &x); err != nil {
				return nil, fmt.Errorf("error parsing JSON file %s: %w", path, err)
			}
		}

		bi.Namespaces[ast.DefaultRootDocument.String()] = append(
			bi.Namespaces[ast.DefaultRootDocument.String()],
			filepath.Clean(path),
		)
	}

	return bi, nil
}

func bundleOrDirInfoForRegoVersion(regoVersion ast.RegoVersion, path string, includeAnnotations bool) (*Info, error) {
	b, err := loader.NewFileLoader().
		WithRegoVersion(regoVersion).
		WithSkipBundleVerification(true).
		WithBundleLazyLoadingMode(true). // Bundle lazy loading mode skips parsing data files
		WithProcessAnnotation(true).     // Always process annotations, for enriching namespace listing
		AsBundle(path)
	if err != nil {
		return nil, err
	}

	bi := &Info{Manifest: &b.Manifest}

	namespaces := make(map[string][]string, len(b.Modules))
	modules := make([]*ast.Module, 0, len(b.Modules))
	for _, m := range b.Modules {
		namespaces[m.Parsed.Package.Path.String()] = append(namespaces[m.Parsed.Package.Path.String()], filepath.Clean(m.Path))
		modules = append(modules, m.Parsed)
	}
	bi.Namespaces = namespaces

	if includeAnnotations {
		as, errs := ast.BuildAnnotationSet(modules)
		if len(errs) > 0 {
			return nil, errs
		}
		flattened := as.Flatten()

		for _, wr := range bi.Manifest.WasmResolvers {
			if as := wr.Annotations; len(as) > 0 {
				path, err := ast.PtrRef(ast.DefaultRootDocument, wr.Entrypoint)
				if err != nil {
					return nil, fmt.Errorf("failed to parse Wasm entrypoint in manifest: %s", err)
				}
				for _, a := range as {
					ar := ast.NewAnnotationsRef(a)
					ar.Path = path
					ar.Location = ast.NewLocation(nil, wr.Module, 0, 0)
					flattened = flattened.Insert(ar)
				}
			}
		}

		bi.Annotations = flattened
	}

	err = bi.getBundleDataWasmAndSignatures(path)
	if err != nil {
		return nil, err
	}

	wasmModules := make([]map[string]any, 0, len(b.WasmModules))
	for _, w := range b.WasmModules {
		wasmModule := map[string]any{
			"url":  w.URL,
			"path": w.Path,
		}

		var entrypoints []string
		for _, r := range w.Entrypoints {
			entrypoints = append(entrypoints, r.String())
		}
		wasmModule["entrypoints"] = entrypoints

		wasmModules = append(wasmModules, wasmModule)
	}
	bi.WasmModules = wasmModules

	moduleMap := make(map[string]*ast.Module, len(b.Modules))
	for _, f := range b.Modules {
		moduleMap[f.URL] = f.Parsed
	}

	c := ast.NewCompiler().
		WithAllowUndefinedFunctionCalls(true)
	c.Compile(moduleMap)
	if c.Failed() {
		return bi, c.Errors
	}

	bi.Required = c.Required

	return bi, nil
}

func (bi *Info) getBundleDataWasmAndSignatures(name string) error {

	load, err := initload.WalkPaths([]string{name}, nil, true)
	if err != nil {
		return err
	}

	if len(load.BundlesLoader) == 0 || len(load.BundlesLoader) > 1 {
		return errors.New("expected information on one bundle only but got none or multiple")
	}

	bl := load.BundlesLoader[0]
	descriptors := []*bundle.Descriptor{}

	for {
		f, err := bl.DirectoryLoader.NextFile()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("bundle read failed: %w", err)
		}

		if strings.HasSuffix(f.Path(), bundle.SignaturesFile) {
			var buf bytes.Buffer
			n, err := f.Read(&buf, bundle.DefaultSizeLimitBytes+1)
			f.Close()

			if err != nil && err != io.EOF {
				return err
			} else if err == nil && n >= bundle.DefaultSizeLimitBytes {
				return fmt.Errorf("bundle file exceeded max size (%v bytes)", bundle.DefaultSizeLimitBytes)
			}

			var signatures bundle.SignaturesConfig
			if err := util.NewJSONDecoder(&buf).Decode(&signatures); err != nil {
				return fmt.Errorf("bundle load failed on signatures decode: %w", err)
			}
			bi.Signatures = signatures
		}

		if filepath.Base(f.Path()) == "data.json" || filepath.Base(f.Path()) == "data.yaml" {
			descriptors = append(descriptors, f)
		}
	}

	for _, f := range descriptors {
		path := filepath.Clean(f.Path())
		key := strings.Split(strings.TrimPrefix(path, string(os.PathSeparator)), string(os.PathSeparator))

		value := path
		if bl.IsDir {
			value = filepath.Clean(f.URL())
		}

		if len(key) > 1 {
			key = key[:len(key)-1] // ignore file name ie. data.json / data.yaml
			path := fmt.Sprintf("%v.%v", ast.DefaultRootDocument, strings.Join(key, "."))
			bi.Namespaces[path] = append(bi.Namespaces[path], value)
		} else {
			bi.Namespaces[ast.DefaultRootDocument.String()] = append(bi.Namespaces[ast.DefaultRootDocument.String()], value) // data file at bundle root
		}
	}

	for _, item := range bi.Manifest.WasmResolvers {
		key := strings.Split(strings.TrimPrefix(item.Entrypoint, "/"), "/")
		path := fmt.Sprintf("%v.%v", ast.DefaultRootDocument, strings.Join(key, "."))
		bi.Namespaces[path] = append(bi.Namespaces[path], item.Module)
	}

	return nil
}

func fileInfoForRegoVersion(regoVersion ast.RegoVersion, path string, includeAnnotations bool) (*Info, error) {
	res, err := loader.NewFileLoader().
		WithRegoVersion(regoVersion).
		WithSkipBundleVerification(true).
		WithProcessAnnotation(true). // Always process annotations, for enriching namespace listing
		All([]string{path})
	if err != nil {
		return nil, err
	}
	bi := &Info{
		Namespaces: make(map[string][]string, len(res.Modules)),
	}

	moduleMap := make(map[string]*ast.Module, len(res.Modules))

	for _, m := range res.Modules {
		bi.Namespaces[m.Parsed.Package.Path.String()] = append(
			bi.Namespaces[m.Parsed.Package.Path.String()],
			filepath.Clean(m.Name),
		)
		moduleMap[m.Name] = m.Parsed
	}

	if includeAnnotations {
		as, errs := ast.BuildAnnotationSet(util.Values(moduleMap))
		if len(errs) > 0 {
			return nil, errs
		}

		bi.Annotations = as.Flatten()
	}

	c := ast.NewCompiler().
		WithAllowUndefinedFunctionCalls(true)
	c.Compile(moduleMap)
	if c.Failed() {
		return bi, c.Errors
	}

	bi.Required = c.Required

	return bi, nil
}
