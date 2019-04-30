// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle implements bundle loading.
package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
)

// Common file extensions and file names.
const (
	RegoExt     = ".rego"
	jsonExt     = ".json"
	manifestExt = ".manifest"
	dataFile    = "data.json"
)

const bundleLimitBytes = (1024 * 1024 * 1024) + 1 // limit bundle reads to 1GB to protect against gzip bombs

var manifestPath = []string{"system", "bundle", "manifest"}

// Bundle represents a loaded bundle. The bundle can contain data and policies.
type Bundle struct {
	Manifest Manifest
	Data     map[string]interface{}
	Modules  []ModuleFile
}

// Manifest represents the manifest from a bundle. The manifest may contain
// metadata such as the bundle revision.
type Manifest struct {
	Revision string    `json:"revision"`
	Roots    *[]string `json:"roots,omitempty"`
}

// Init initializes the manifest. If you instantiate a manifest
// manually, call Init to ensure that the roots are set properly.
func (m *Manifest) Init() {
	if m.Roots == nil {
		defaultRoots := []string{""}
		m.Roots = &defaultRoots
	}
}

func (m *Manifest) validateAndInjectDefaults(b Bundle) error {

	m.Init()

	// Validate roots in bundle.
	roots := *m.Roots
	for i := range roots {
		roots[i] = strings.Trim(roots[i], "/")
	}

	for i := 0; i < len(roots)-1; i++ {
		for j := i + 1; j < len(roots); j++ {
			if strings.HasPrefix(roots[i], roots[j]) || strings.HasPrefix(roots[j], roots[i]) {
				return fmt.Errorf("manifest has overlapped roots: %v and %v", roots[i], roots[j])
			}
		}
	}

	// Validate modules in bundle.
	for _, module := range b.Modules {
		found := false
		if path, err := module.Parsed.Package.Path.Ptr(); err == nil {
			for i := range roots {
				if strings.HasPrefix(path, roots[i]) {
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("manifest roots do not permit '%v' in %v", module.Parsed.Package, module.Path)
		}
	}

	// Validate data in bundle.
	return dfs(b.Data, "", func(path string, node interface{}) (bool, error) {
		path = strings.Trim(path, "/")
		for i := range roots {
			if strings.HasPrefix(path, roots[i]) {
				return true, nil
			}
		}
		if _, ok := node.(map[string]interface{}); ok {
			for i := range roots {
				if strings.HasPrefix(roots[i], path) {
					return false, nil
				}
			}
		}
		return false, fmt.Errorf("manifest roots do not permit data at path %v", path)
	})
}

// ModuleFile represents a single module contained a bundle.
type ModuleFile struct {
	Path   string
	Raw    []byte
	Parsed *ast.Module
}

// Reader contains the reader to load the bundle from.
type Reader struct {
	r                     io.Reader
	includeManifestInData bool
}

// NewReader returns a new Reader.
func NewReader(r io.Reader) *Reader {
	nr := Reader{}
	nr.r = r
	return &nr
}

// IncludeManifestInData sets whether the manifest metadata should be
// included in the bundle's data.
func (r *Reader) IncludeManifestInData(includeManifestInData bool) *Reader {
	r.includeManifestInData = includeManifestInData
	return r
}

// Read returns a new Bundle loaded from the reader.
func (r *Reader) Read() (Bundle, error) {

	var bundle Bundle

	bundle.Data = map[string]interface{}{}

	gr, err := gzip.NewReader(r.r)
	if err != nil {
		return bundle, errors.Wrap(err, "bundle read failed")
	}

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return bundle, errors.Wrap(err, "bundle read failed")
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		var buf bytes.Buffer
		n, err := io.CopyN(&buf, tr, bundleLimitBytes)
		if err != nil && err != io.EOF {
			return bundle, err
		} else if err == nil && n >= bundleLimitBytes {
			return bundle, fmt.Errorf("bundle exceeded max size (%v bytes)", bundleLimitBytes-1)
		}

		path := header.Name

		if strings.HasSuffix(path, RegoExt) {
			module, err := ast.ParseModule(path, buf.String())
			if err != nil {
				return bundle, errors.Wrap(err, "bundle load failed")
			}
			if module == nil {
				return bundle, errors.Wrap(fmt.Errorf("module '%s' is empty", path), "bundle load failed")
			}

			file := ModuleFile{
				Path:   path,
				Raw:    buf.Bytes(),
				Parsed: module,
			}
			bundle.Modules = append(bundle.Modules, file)

		} else if filepath.Base(path) == dataFile {
			var value interface{}
			if err := util.NewJSONDecoder(&buf).Decode(&value); err != nil {
				return bundle, errors.Wrapf(err, "bundle load failed on %v", path)
			}
			// Remove leading / and . characters from the directory path. If the bundle
			// was written with OPA then the paths will contain a leading slash. On the
			// other hand, if the path is empty, filepath.Dir will return '.'.
			dirpath := strings.TrimLeft(filepath.Dir(path), "/.")
			var key []string
			if dirpath != "" {
				key = strings.Split(dirpath, "/")
			}
			if err := bundle.insert(key, value); err != nil {
				return bundle, errors.Wrapf(err, "bundle load failed on %v", path)
			}

		} else if strings.HasSuffix(path, manifestExt) {
			if err := util.NewJSONDecoder(&buf).Decode(&bundle.Manifest); err != nil {
				return bundle, errors.Wrap(err, "bundle load failed on manifest decode")
			}
		}
	}

	if err := bundle.Manifest.validateAndInjectDefaults(bundle); err != nil {
		return bundle, err
	}

	if r.includeManifestInData {
		var metadata map[string]interface{}

		b, err := json.Marshal(&bundle.Manifest)
		if err != nil {
			return bundle, errors.Wrap(err, "bundle load failed on manifest marshal")
		}

		err = util.UnmarshalJSON(b, &metadata)
		if err != nil {
			return bundle, errors.Wrap(err, "bundle load failed on manifest unmarshal")
		}

		if err := bundle.insert(manifestPath, metadata); err != nil {
			return bundle, errors.Wrapf(err, "bundle load failed on %v", manifestPath)
		}
	}

	return bundle, nil
}

// Write serializes the Bundle and writes it to w.
func Write(w io.Writer, bundle Bundle) error {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(bundle.Data); err != nil {
		return err
	}

	if err := writeFile(tw, "data.json", buf.Bytes()); err != nil {
		return err
	}

	for _, module := range bundle.Modules {
		if err := writeFile(tw, module.Path, module.Raw); err != nil {
			return err
		}
	}

	if err := writeManifest(tw, bundle); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	return gw.Close()
}

func writeManifest(tw *tar.Writer, bundle Bundle) error {

	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(bundle.Manifest); err != nil {
		return err
	}

	return writeFile(tw, manifestExt, buf.Bytes())
}

// Equal returns true if this bundle's contents equal the other bundle's
// contents.
func (b Bundle) Equal(other Bundle) bool {
	if !reflect.DeepEqual(b.Data, other.Data) {
		return false
	}
	if len(b.Modules) != len(other.Modules) {
		return false
	}
	for i := range b.Modules {
		if b.Modules[i].Path != other.Modules[i].Path {
			return false
		}
		if !b.Modules[i].Parsed.Equal(other.Modules[i].Parsed) {
			return false
		}
		if !bytes.Equal(b.Modules[i].Raw, other.Modules[i].Raw) {
			return false
		}
	}
	return true
}

func (b *Bundle) insert(key []string, value interface{}) error {
	if len(key) == 0 {
		obj, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("root value must be object")
		}
		b.Data = obj
		return nil
	}

	obj, err := b.mkdir(key[:len(key)-1])
	if err != nil {
		return err
	}

	obj[key[len(key)-1]] = value
	return nil
}

func (b *Bundle) mkdir(key []string) (map[string]interface{}, error) {
	obj := b.Data
	for i := 0; i < len(key); i++ {
		node, ok := obj[key[i]]
		if !ok {
			node = map[string]interface{}{}
			obj[key[i]] = node
		}
		obj, ok = node.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("non-leaf value must be object")
		}
	}
	return obj, nil
}

func writeFile(tw *tar.Writer, path string, bs []byte) error {

	hdr := &tar.Header{
		Name:     "/" + strings.TrimLeft(path, "/"),
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(bs)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	_, err := tw.Write(bs)
	return err
}

func dfs(value interface{}, path string, fn func(string, interface{}) (bool, error)) error {
	if stop, err := fn(path, value); err != nil {
		return err
	} else if stop {
		return nil
	}
	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	for key := range obj {
		if err := dfs(obj[key], path+"/"+key, fn); err != nil {
			return err
		}
	}
	return nil
}
