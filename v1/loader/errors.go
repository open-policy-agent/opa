// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package loader

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
)

// Errors is a wrapper for multiple loader errors.
type Errors []error

func (e Errors) Error() string {
	if len(e) == 0 {
		return "no error(s)"
	}
	if len(e) == 1 {
		return "1 error occurred during loading: " + e[0].Error()
	}
	buf := util.Map(e, error.Error)
	return fmt.Sprintf("%v errors occurred during loading:\n", len(e)) + strings.Join(buf, "\n")
}

func (e *Errors) add(err error) {
	if errs, ok := err.(ast.Errors); ok {
		for i := range errs {
			*e = append(*e, errs[i])
		}
	} else {
		*e = append(*e, err)
	}
}

type unsupportedDocumentType string

func (path unsupportedDocumentType) Error() string {
	return string(path) + ": document must be of type object"
}

type unrecognizedFile string

func (path unrecognizedFile) Error() string {
	return string(path) + ": can't recognize file type"
}

func isUnrecognizedFile(err error) bool {
	_, ok := err.(unrecognizedFile)
	return ok
}

type mergeError struct {
	path     string
	conflict string
	keys     []string
	other    string
}

func (e mergeError) Error() string {
	if e.conflict == "" {
		return e.path + ": merge error"
	}
	msg := e.path + ": merge error: conflicting values at " + e.conflict
	if e.other != "" {
		return msg + ", also defined in " + e.other
	}
	return msg + ", data from multiple files must not overlap"
}

// loadedFile identifies a file that has been loaded successfully, and where
// under data it was loaded.
type loadedFile struct {
	parent []string
	path   string
	depth  int
}

// withOtherFile attempts to find the first of the previously loaded files that
// defines the conflicting value. Files are loaded again for this, as merging
// modifies the previously loaded documents, which makes it impossible to tell
// afterwards where a value originated. This only runs when loading has failed.
func (e mergeError) withOtherFile(files []loadedFile, f func(*Result, string, int) error) mergeError {
	if len(e.keys) == 0 {
		return e
	}
	for _, file := range files {
		scratch := newResult()
		scratch.path = file.parent
		if err := f(scratch, file.path, file.depth); err != nil {
			continue
		}
		if hasPath(scratch.Documents, e.keys) {
			e.other = file.path
			break
		}
	}
	return e
}

func hasPath(doc map[string]any, keys []string) bool {
	for i, k := range keys {
		v, ok := doc[k]
		if !ok {
			return false
		}
		if i == len(keys)-1 {
			return true
		}
		if doc, ok = v.(map[string]any); !ok {
			return false
		}
	}
	return false
}
