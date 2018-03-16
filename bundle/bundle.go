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
	"reflect"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
)

// Common file extensions and file names.
const (
	RegoExt     = ".rego"
	JSONExt     = ".json"
	DataFileExt = "/data.json"
)

// Bundle represents a loaded bundle. The bundle can contain data and policies.
type Bundle struct {
	Data    map[string]interface{}
	Modules []ModuleFile
}

// ModuleFile represents a single module contained a bundle.
type ModuleFile struct {
	Path   string
	Raw    []byte
	Parsed *ast.Module
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

	if err := tw.Close(); err != nil {
		return err
	}

	return gw.Close()
}

// Read returns a new Bundle loaded from the reader.
func Read(r io.Reader) (Bundle, error) {

	var bundle Bundle

	bundle.Data = map[string]interface{}{}

	gr, err := gzip.NewReader(r)
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
		io.Copy(&buf, tr)
		path := header.Name

		if strings.HasSuffix(path, RegoExt) {
			module, err := ast.ParseModule(path, buf.String())
			if err != nil {
				return bundle, errors.Wrap(err, "bundle load failed")
			}
			file := ModuleFile{
				Path:   path,
				Raw:    buf.Bytes(),
				Parsed: module,
			}
			bundle.Modules = append(bundle.Modules, file)

		} else if strings.HasSuffix(path, DataFileExt) {
			var value interface{}
			if err := util.NewJSONDecoder(&buf).Decode(&value); err != nil {
				return bundle, errors.Wrapf(err, "bundle load failed on %v", path)
			}
			dirpath := strings.Trim(strings.TrimSuffix(path, DataFileExt), "/")
			var key []string
			if dirpath != "" {
				key = strings.Split(dirpath, "/")
			}
			if err := bundle.insert(key, value); err != nil {
				return bundle, errors.Wrapf(err, "bundle load failed on %v", path)
			}
		}
	}

	return bundle, nil
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
