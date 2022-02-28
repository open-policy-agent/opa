// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inspect

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util"
)

// Info represents information about a bundle.
type Info struct {
	Manifest    bundle.Manifest            `json:"manifest,omitempty"`
	Signatures  bundle.SignaturesConfig    `json:"signatures_config,omitempty"`
	WasmModules []map[string]interface{}   `json:"wasm_modules,omitempty"`
	Namespaces  map[string][]NamespaceInfo `json:"namespaces,omitempty"`
	Annotations []*ast.AnnotationsRef      `json:"annotations,omitempty"`
}

type NamespaceInfo struct {
	File          string   `json:"file"`
	Title         string   `json:"title,omitempty"`
	Organizations []string `json:"organizations,omitempty"`
}

func File(path string, includeAnnotations bool, annotationsFilter []string) (*Info, error) {
	b, err := loader.NewFileLoader().
		WithSkipBundleVerification(true).
		WithProcessAnnotation(true). // Always process annotations, for enriching namespace listing
		AsBundle(path)
	if err != nil {
		return nil, err
	}

	bi := &Info{Manifest: b.Manifest}

	namespaces := make(map[string][]NamespaceInfo, len(b.Modules))
	var modules []*ast.Module
	for _, m := range b.Modules {
		ni := NamespaceInfo{
			File: filepath.Clean(m.Path),
		}
		if pa := ast.FindPackageAnnotations(m.Parsed.Annotations); pa != nil {
			// Regular annotations overriding not used for namespace info
			ni.Title = pa.Title
			ni.Organizations = pa.Organizations
		}
		namespaces[m.Parsed.Package.Path.String()] = append(namespaces[m.Parsed.Package.Path.String()], ni)
		modules = append(modules, m.Parsed)
	}
	bi.Namespaces = namespaces

	if includeAnnotations {
		af, err := toRefs(annotationsFilter)
		var errs ast.Errors
		bi.Annotations, errs = ast.GetAnnotations(modules, af...)
		if len(errs) > 0 {
			return nil, err
		}
	}

	err = bi.getBundleDataWasmAndSignatures(path)
	if err != nil {
		return nil, err
	}

	wasmModules := make([]map[string]interface{}, 0, len(b.WasmModules))
	for _, w := range b.WasmModules {
		wasmModule := map[string]interface{}{
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

	return bi, nil
}

func toRefs(strings []string) ([]ast.Ref, error) {
	result := make([]ast.Ref, 0, len(strings))
	for _, s := range strings {
		ref, err := ast.ParseRef(s)
		if err != nil {
			return nil, err
		}
		result = append(result, ref)
	}
	return result, nil
}

func (bi *Info) getBundleDataWasmAndSignatures(name string) error {

	load, err := initload.WalkPaths([]string{name}, nil, true)
	if err != nil {
		return err
	}

	if len(load.BundlesLoader) == 0 || len(load.BundlesLoader) > 1 {
		return fmt.Errorf("expected information on one bundle only but got none or multiple")
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
		key := strings.Split(strings.TrimPrefix(path, "/"), "/")

		value := path
		if bl.IsDir {
			value = f.URL()
		}

		if len(key) > 1 {
			key = key[:len(key)-1] // ignore file name ie. data.json / data.yaml
			path := fmt.Sprintf("%v.%v", ast.DefaultRootDocument, strings.Join(key, "."))
			bi.Namespaces[path] = append(bi.Namespaces[path], NamespaceInfo{File: value})
		} else {
			bi.Namespaces[ast.DefaultRootDocument.String()] = append(bi.Namespaces[ast.DefaultRootDocument.String()], NamespaceInfo{File: value}) // data file at bundle root
		}
	}

	for _, item := range bi.Manifest.WasmResolvers {
		key := strings.Split(strings.TrimPrefix(item.Entrypoint, "/"), "/")
		path := fmt.Sprintf("%v.%v", ast.DefaultRootDocument, strings.Join(key, "."))
		bi.Namespaces[path] = append(bi.Namespaces[path], NamespaceInfo{File: item.Module})
	}

	return nil
}
