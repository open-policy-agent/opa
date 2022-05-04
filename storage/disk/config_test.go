// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v3"
	"github.com/open-policy-agent/opa/logging"
)

func TestNewFromConfig(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "disk_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	for _, tc := range []struct {
		note    string
		config  string
		err     error // gets unwrapped
		nothing bool  // returns no disk options?
	}{
		{
			note:    "no storage section",
			config:  "",
			nothing: true,
		},
		{
			note: "successful init, no partitions",
			config: `
storage:
  disk:
    directory: "` + tmpdir + `"
`,
		},
		{
			note: "successful init, valid partitions",
			config: `
storage:
  disk:
    directory: "` + tmpdir + `"
    partitions:
    - /foo/bar
    - /baz
`,
		},
		{
			note: "partitions invalid",
			config: `
storage:
  disk:
    directory: "` + tmpdir + `"
    partitions:
    - /foo/bar
    - baz
`,
			err: ErrInvalidPartitionPath,
		},
		{
			note: "directory does not exist",
			config: `
storage:
  disk:
    directory: "` + tmpdir + `/foobar"
`,
			err: os.ErrNotExist,
		},
		{
			note: "auto-create directory, does not exist",
			config: `
storage:
  disk:
    auto_create: true
    directory: "` + tmpdir + `/foobar"
`,
		},
		{
			note: "auto-create directory, does already exist", // could be the second run
			config: `
storage:
  disk:
    auto_create: true
    directory: "` + tmpdir + `"
`,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			d, err := OptionsFromConfig([]byte(tc.config), "id")
			if !errors.Is(err, tc.err) {
				t.Errorf("err: expected %v, got %v", tc.err, err)
			}
			if tc.nothing && d != nil {
				t.Errorf("expected no disk options, got %v", d)
			}
		})
	}
}

func TestDataDirPrefix(t *testing.T) {
	ctx := context.Background()
	tmpdir, err := ioutil.TempDir("", "disk_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	d, err := New(ctx, logging.NewNoOpLogger(), nil, Options{
		Dir: tmpdir,
	})
	if err != nil {
		t.Fatal(err)
	}
	d.Close(ctx)

	dir := filepath.Join(tmpdir, "data")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("stat %v: %v", dir, err)
	}
	files, err := os.ReadDir(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	// We currently only expect a single directory here: "data"
	for _, file := range files {
		if file.Name() != "data" {
			t.Errorf("unexpected file in dir: %v", file.Name())
		}
	}
}

func TestBadgerConfigFromOptions(t *testing.T) {
	type check func(*testing.T, badger.Options)
	checks := func(c ...check) []check {
		return c
	}
	valueDir := func(exp string) check {
		return func(t *testing.T, o badger.Options) {
			if act := o.ValueDir; act != exp {
				t.Errorf("ValueDir: expected %v, got %v", exp, act)
			}
		}
	}
	dir := func(exp string) check {
		return func(t *testing.T, o badger.Options) {
			if act := o.Dir; act != exp {
				t.Errorf("Dir: expected %v, got %v", exp, act)
			}
		}
	}
	conflict := func(exp bool) check {
		return func(t *testing.T, o badger.Options) {
			if act := o.DetectConflicts; act != exp {
				t.Errorf("DetectConflicts: expected %v, got %v", exp, act)
			}
		}
	}
	nummemtables := func(exp int) check {
		return func(t *testing.T, o badger.Options) {
			if act := o.NumMemtables; act != exp {
				t.Errorf("Dir: expected %v, got %v", exp, act)
			}
		}
	}
	numversionstokeep := func(exp int) check {
		return func(t *testing.T, o badger.Options) {
			if act := o.NumVersionsToKeep; act != exp {
				t.Errorf("Dir: expected %v, got %v", exp, act)
			}
		}
	}

	tests := []struct {
		note   string
		opts   Options
		checks []check
	}{
		{
			"defaults",
			Options{
				Dir: "foo",
			},
			checks(
				valueDir("foo/data"),
				dir("foo/data"),
				conflict(false),
			),
		},
		{
			"valuedir+dir override",
			Options{
				Dir:    "foo",
				Badger: `valuedir="baz"; dir="quz"`,
			},
			checks(
				valueDir("foo/data"),
				dir("foo/data"),
			),
		},
		{
			"conflict detection override",
			Options{
				Dir:    "foo",
				Badger: `detectconflicts=true`,
			},
			checks(conflict(false)),
		},
		{
			"two valid overrides", // NOTE(sr): This is just one example
			Options{
				Dir:    "foo",
				Badger: `nummemtables=123; numversionstokeep=123`,
			},
			checks(
				nummemtables(123),
				numversionstokeep(123),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			act, _ := badgerConfigFromOptions(tc.opts)
			for _, check := range tc.checks {
				check(t, act)
			}
		})
	}
}
