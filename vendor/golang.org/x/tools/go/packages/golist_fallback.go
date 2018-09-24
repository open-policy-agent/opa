// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packages

import (
	"encoding/json"
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/internal/cgo"
)

// TODO(matloob): Delete this file once Go 1.12 is released.

// This file provides backwards compatibility support for
// loading for versions of Go earlier than 1.10.4. This support is meant to
// assist with migration to the Package API until there's
// widespread adoption of these newer Go versions.
// This support will be removed once Go 1.12 is released
// in Q1 2019.

func golistDriverFallback(cfg *Config, words ...string) (*driverResponse, error) {
	original, deps, err := getDeps(cfg, words...)
	if err != nil {
		return nil, err
	}

	var tmpdir string // used for generated cgo files

	var response driverResponse
	addPackage := func(p *jsonPackage) {
		if p.Name == "" {
			return
		}

		id := p.ImportPath
		isRoot := original[id] != nil
		pkgpath := id

		if pkgpath == "unsafe" {
			p.GoFiles = nil // ignore fake unsafe.go file
		}

		importMap := func(importlist []string) map[string]*Package {
			importMap := make(map[string]*Package)
			for _, id := range importlist {

				if id == "C" {
					for _, path := range []string{"unsafe", "syscall", "runtime/cgo"} {
						if pkgpath != path && importMap[path] == nil {
							importMap[path] = &Package{ID: path}
						}
					}
					continue
				}
				importMap[vendorlessPath(id)] = &Package{ID: id}
			}
			return importMap
		}
		compiledGoFiles := absJoin(p.Dir, p.GoFiles)
		// Use a function to simplify control flow. It's just a bunch of gotos.
		var cgoErrors []error
		var outdir string
		getOutdir := func() (string, error) {
			if outdir != "" {
				return outdir, nil
			}
			if tmpdir == "" {
				if tmpdir, err = ioutil.TempDir("", "gopackages"); err != nil {
					return "", err
				}
			}
			// Add a "go-build" component to the path to make the tests think the files are in the cache.
			// This allows the same test to test the pre- and post-Go 1.11 go list logic because the Go 1.11
			// go list generates test mains in the cache, and the test code knows not to rely on paths in the
			// cache to stay stable.
			outdir = filepath.Join(tmpdir, "go-build", strings.Replace(p.ImportPath, "/", "_", -1))
			if err := os.MkdirAll(outdir, 0755); err != nil {
				outdir = ""
				return "", err
			}
			return outdir, nil
		}
		processCgo := func() bool {
			// Suppress any cgo errors. Any relevant errors will show up in typechecking.
			// TODO(matloob): Skip running cgo if Mode < LoadTypes.
			outdir, err := getOutdir()
			if err != nil {
				cgoErrors = append(cgoErrors, err)
				return false
			}
			files, _, err := runCgo(p.Dir, outdir, cfg.Env)
			if err != nil {
				cgoErrors = append(cgoErrors, err)
				return false
			}
			compiledGoFiles = append(compiledGoFiles, files...)
			return true
		}
		if len(p.CgoFiles) == 0 || !processCgo() {
			compiledGoFiles = append(compiledGoFiles, absJoin(p.Dir, p.CgoFiles)...) // Punt to typechecker.
		}
		if isRoot {
			response.Roots = append(response.Roots, id)
		}
		response.Packages = append(response.Packages, &Package{
			ID:              id,
			Name:            p.Name,
			GoFiles:         absJoin(p.Dir, p.GoFiles, p.CgoFiles),
			CompiledGoFiles: compiledGoFiles,
			OtherFiles:      absJoin(p.Dir, otherFiles(p)...),
			PkgPath:         pkgpath,
			Imports:         importMap(p.Imports),
			// TODO(matloob): set errors on the Package to cgoErrors
		})
		if cfg.Tests {
			testID := fmt.Sprintf("%s [%s.test]", id, id)
			if len(p.TestGoFiles) > 0 || len(p.XTestGoFiles) > 0 {
				if isRoot {
					response.Roots = append(response.Roots, testID)
				}
				testPkg := &Package{
					ID:              testID,
					Name:            p.Name,
					GoFiles:         absJoin(p.Dir, p.GoFiles, p.CgoFiles, p.TestGoFiles),
					CompiledGoFiles: append(compiledGoFiles, absJoin(p.Dir, p.TestGoFiles)...),
					OtherFiles:      absJoin(p.Dir, otherFiles(p)...),
					PkgPath:         pkgpath,
					Imports:         importMap(append(p.Imports, p.TestImports...)),
					// TODO(matloob): set errors on the Package to cgoErrors
				}
				response.Packages = append(response.Packages, testPkg)
				var xtestPkg *Package
				if len(p.XTestGoFiles) > 0 {
					xtestID := fmt.Sprintf("%s_test [%s.test]", id, id)
					if isRoot {
						response.Roots = append(response.Roots, xtestID)
					}
					// Rewrite import to package under test to refer to test variant.
					imports := importMap(p.XTestImports)
					for imp := range imports {
						if imp == p.ImportPath {
							imports[imp] = &Package{ID: testID}
							break
						}
					}
					xtestPkg = &Package{
						ID:              xtestID,
						Name:            p.Name + "_test",
						GoFiles:         absJoin(p.Dir, p.XTestGoFiles),
						CompiledGoFiles: absJoin(p.Dir, p.XTestGoFiles),
						PkgPath:         pkgpath + "_test",
						Imports:         imports,
					}
					response.Packages = append(response.Packages, xtestPkg)
				}
				// testmain package
				testmainID := id + ".test"
				if isRoot {
					response.Roots = append(response.Roots, testmainID)
				}
				imports := map[string]*Package{}
				imports[testPkg.PkgPath] = &Package{ID: testPkg.ID}
				if xtestPkg != nil {
					imports[xtestPkg.PkgPath] = &Package{ID: xtestPkg.ID}
				}
				testmainPkg := &Package{
					ID:      testmainID,
					Name:    "main",
					PkgPath: testmainID,
					Imports: imports,
				}
				response.Packages = append(response.Packages, testmainPkg)
				outdir, err := getOutdir()
				if err != nil {
					testmainPkg.Errors = append(testmainPkg.Errors,
						Error{"-", fmt.Sprintf("failed to generate testmain: %v", err)})
					return
				}
				testmain := filepath.Join(outdir, "testmain.go")
				extradeps, err := generateTestmain(testmain, testPkg, xtestPkg)
				if err != nil {
					testmainPkg.Errors = append(testmainPkg.Errors,
						Error{"-", fmt.Sprintf("failed to generate testmain: %v", err)})
				}
				deps = append(deps, extradeps...)
				for _, imp := range extradeps { // testing, testing/internal/testdeps, and maybe os
					imports[imp] = &Package{ID: imp}
				}
				testmainPkg.GoFiles = []string{testmain}
				testmainPkg.CompiledGoFiles = []string{testmain}
			}
		}
	}

	for _, pkg := range original {
		addPackage(pkg)
	}
	if cfg.Mode < LoadImports || len(deps) == 0 {
		return &response, nil
	}

	buf, err := golist(cfg, golistArgsFallback(cfg, deps))
	if err != nil {
		return nil, err
	}

	// Decode the JSON and convert it to Package form.
	for dec := json.NewDecoder(buf); dec.More(); {
		p := new(jsonPackage)
		if err := dec.Decode(p); err != nil {
			return nil, fmt.Errorf("JSON decoding failed: %v", err)
		}

		addPackage(p)
	}

	// TODO(matloob): Is this the right ordering?
	sort.SliceStable(response.Packages, func(i, j int) bool {
		return response.Packages[i].PkgPath < response.Packages[j].PkgPath
	})

	return &response, nil
}

// vendorlessPath returns the devendorized version of the import path ipath.
// For example, VendorlessPath("foo/bar/vendor/a/b") returns "a/b".
// Copied from golang.org/x/tools/imports/fix.go.
func vendorlessPath(ipath string) string {
	// Devendorize for use in import statement.
	if i := strings.LastIndex(ipath, "/vendor/"); i >= 0 {
		return ipath[i+len("/vendor/"):]
	}
	if strings.HasPrefix(ipath, "vendor/") {
		return ipath[len("vendor/"):]
	}
	return ipath
}

// getDeps runs an initial go list to determine all the dependency packages.
func getDeps(cfg *Config, words ...string) (originalSet map[string]*jsonPackage, deps []string, err error) {
	buf, err := golist(cfg, golistArgsFallback(cfg, words))
	if err != nil {
		return nil, nil, err
	}

	depsSet := make(map[string]bool)
	originalSet = make(map[string]*jsonPackage)
	var testImports []string

	// Extract deps from the JSON.
	for dec := json.NewDecoder(buf); dec.More(); {
		p := new(jsonPackage)
		if err := dec.Decode(p); err != nil {
			return nil, nil, fmt.Errorf("JSON decoding failed: %v", err)
		}

		originalSet[p.ImportPath] = p
		for _, dep := range p.Deps {
			depsSet[dep] = true
		}
		if cfg.Tests {
			// collect the additional imports of the test packages.
			pkgTestImports := append(p.TestImports, p.XTestImports...)
			for _, imp := range pkgTestImports {
				if depsSet[imp] {
					continue
				}
				depsSet[imp] = true
				testImports = append(testImports, imp)
			}
		}
	}
	// Get the deps of the packages imported by tests.
	if len(testImports) > 0 {
		buf, err = golist(cfg, golistArgsFallback(cfg, testImports))
		if err != nil {
			return nil, nil, err
		}
		// Extract deps from the JSON.
		for dec := json.NewDecoder(buf); dec.More(); {
			p := new(jsonPackage)
			if err := dec.Decode(p); err != nil {
				return nil, nil, fmt.Errorf("JSON decoding failed: %v", err)
			}
			for _, dep := range p.Deps {
				depsSet[dep] = true
			}
		}
	}

	for orig := range originalSet {
		delete(depsSet, orig)
	}

	deps = make([]string, 0, len(depsSet))
	for dep := range depsSet {
		deps = append(deps, dep)
	}
	sort.Strings(deps) // ensure output is deterministic
	return originalSet, deps, nil
}

func golistArgsFallback(cfg *Config, words []string) []string {
	fullargs := []string{"list", "-e", "-json"}
	fullargs = append(fullargs, cfg.BuildFlags...)
	fullargs = append(fullargs, "--")
	fullargs = append(fullargs, words...)
	return fullargs
}

func runCgo(pkgdir, tmpdir string, env []string) (files, displayfiles []string, err error) {
	// Use go/build to open cgo files and determine the cgo flags, etc, from them.
	// This is tricky so it's best to avoid reimplementing as much as we can, and
	// we plan to delete this support once Go 1.12 is released anyways.
	// TODO(matloob): This isn't completely correct because we're using the Default
	// context. Perhaps we should more accurately fill in the context.
	bp, err := build.ImportDir(pkgdir, build.ImportMode(0))
	if err != nil {
		return nil, nil, err
	}
	for _, ev := range env {
		if v := strings.TrimPrefix(ev, "CGO_CPPFLAGS"); v != ev {
			bp.CgoCPPFLAGS = append(bp.CgoCPPFLAGS, strings.Fields(v)...)
		} else if v := strings.TrimPrefix(ev, "CGO_CFLAGS"); v != ev {
			bp.CgoCFLAGS = append(bp.CgoCFLAGS, strings.Fields(v)...)
		} else if v := strings.TrimPrefix(ev, "CGO_CXXFLAGS"); v != ev {
			bp.CgoCXXFLAGS = append(bp.CgoCXXFLAGS, strings.Fields(v)...)
		} else if v := strings.TrimPrefix(ev, "CGO_LDFLAGS"); v != ev {
			bp.CgoLDFLAGS = append(bp.CgoLDFLAGS, strings.Fields(v)...)
		}
	}
	return cgo.Run(bp, pkgdir, tmpdir, true)
}
