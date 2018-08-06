// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package basedir finds templates and static files associated with a binary.
package basedir

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Find locates a directory for the given package.
// pkg should be the directory that contains the templates and/or static directories.
// If pkg cannot be found, an empty string will be returned.
func Find(pkg string) string {
	cmd := exec.Command("go", "list", "-e", "-f", "{{.Dir}}", pkg)
	if out, err := cmd.Output(); err == nil && len(out) > 0 {
		return string(bytes.TrimRight(out, "\r\n"))
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = defaultGOPATH()
	}
	if gopath != "" {
		for _, dir := range strings.Split(gopath, ":") {
			p := filepath.Join(dir, pkg)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// Copied from go/build/build.go
func defaultGOPATH() string {
	env := "HOME"
	if runtime.GOOS == "windows" {
		env = "USERPROFILE"
	} else if runtime.GOOS == "plan9" {
		env = "home"
	}
	if home := os.Getenv(env); home != "" {
		def := filepath.Join(home, "go")
		if filepath.Clean(def) == filepath.Clean(runtime.GOROOT()) {
			// Don't set the default GOPATH to GOROOT,
			// as that will trigger warnings from the go tool.
			return ""
		}
		return def
	}
	return ""
}
