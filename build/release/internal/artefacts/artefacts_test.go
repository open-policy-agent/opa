package artefacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "plain", in: "1.19.0", want: "1.19.0"},
		{name: "leading v stripped", in: "v1.19.0", want: "1.19.0"},
		{name: "surrounding space", in: "  v1.19.0 ", want: "1.19.0"},
		{name: "prerelease", in: "1.19.0-rc1", want: "1.19.0-rc1"},
		{name: "dev suffix", in: "1.19.0-dev", want: "1.19.0-dev"},
		{name: "double digit parts", in: "10.20.30", want: "10.20.30"},
		{name: "empty", in: "", wantErr: "version is required"},
		{name: "space only", in: "   ", wantErr: "version is required"},
		{name: "missing patch", in: "1.19", wantErr: "invalid version"},
		{name: "trailing dot", in: "1.19.0.", wantErr: "invalid version"},
		{name: "not a version", in: "latest", wantErr: "invalid version"},
		{name: "prerelease with bad chars", in: "1.19.0-rc 1", wantErr: "invalid version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseVersion(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ParseVersion(%q) error = %v, want it to contain %q", tc.in, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ParseVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

const versionGo = `// Copyright 2016 The OPA Authors.
package version

import "runtime"

var Version = "1.19.0-dev"

// GoVersion is the version of Go this was built with
var GoVersion = runtime.Version()
`

func TestSetVersion(t *testing.T) {
	for _, tc := range []struct {
		name         string
		content      string
		version      string
		wantPrevious string
		wantContains string
		wantErr      string
	}{
		{
			name:         "replaces dev version",
			content:      versionGo,
			version:      "1.19.0",
			wantPrevious: "1.19.0-dev",
			wantContains: `var Version = "1.19.0"` + "\n",
		},
		{
			name:         "idempotent when already set",
			content:      strings.Replace(versionGo, "1.19.0-dev", "1.19.0", 1),
			version:      "1.19.0",
			wantPrevious: "1.19.0",
			wantContains: `var Version = "1.19.0"` + "\n",
		},
		{
			name:    "no assignment is an error",
			content: "package version\n\nconst Version = \"1.0.0\"\n",
			version: "1.19.0",
			wantErr: "no 'var Version",
		},
		{
			name:    "two assignments is an error",
			content: versionGo + "\nvar Version = \"other\"\n",
			version: "1.19.0",
			wantErr: "expected exactly one",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, previous, err := SetVersion(tc.content, tc.version)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if previous != tc.wantPrevious {
				t.Errorf("previous = %q, want %q", previous, tc.wantPrevious)
			}
			if !strings.Contains(got, tc.wantContains) {
				t.Errorf("result does not contain %q:\n%s", tc.wantContains, got)
			}
			// Everything except the Version line must be byte-identical.
			if gotRest, wantRest := withoutVersionLine(got), withoutVersionLine(tc.content); gotRest != wantRest {
				t.Errorf("SetVersion changed more than the Version line:\n--- got ---\n%s\n--- want ---\n%s", gotRest, wantRest)
			}
		})
	}
}

func withoutVersionLine(content string) string {
	var kept []string
	for _, l := range strings.Split(content, "\n") {
		if strings.HasPrefix(l, "var Version = ") {
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}

func TestSnapshotName(t *testing.T) {
	for _, tc := range []struct{ version, want string }{
		{"1.19.0", "capabilities/v1.19.0.json"},
		{"1.19.0-rc1", "capabilities/v1.19.0-rc1.json"},
	} {
		if got := SnapshotName(tc.version); got != tc.want {
			t.Errorf("SnapshotName(%q) = %q, want %q", tc.version, got, tc.want)
		}
	}
}

// fakeRepo builds the minimum tree Prepare touches when generation is skipped.
func fakeRepo(t *testing.T, capabilities string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(VersionFile)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, VersionFile), []byte(versionGo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, CapabilitiesFile), []byte(capabilities), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestPrepare_SkipGenerate(t *testing.T) {
	root := fakeRepo(t, `{"builtins":[]}`)

	res, err := Prepare(Options{RepoRoot: root, Version: "v1.19.0", SkipGenerate: true})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if res.PreviousVersion != "1.19.0-dev" || !res.VersionFileChanged {
		t.Errorf("version result = %+v, want previous 1.19.0-dev and changed", res)
	}
	if got := read(t, filepath.Join(root, VersionFile)); !strings.Contains(got, `var Version = "1.19.0"`) {
		t.Errorf("version.go not bumped:\n%s", got)
	}
	if res.CapabilitiesPath != "capabilities/v1.19.0.json" {
		t.Errorf("CapabilitiesPath = %q", res.CapabilitiesPath)
	}
	if res.CapabilitiesOverwritten {
		t.Error("CapabilitiesOverwritten should be false for a new snapshot")
	}
	if got := read(t, filepath.Join(root, "capabilities", "v1.19.0.json")); got != `{"builtins":[]}` {
		t.Errorf("snapshot content = %q", got)
	}
	if len(res.Generated) != 0 {
		t.Errorf("Generated = %v, want empty with --skip-generate", res.Generated)
	}
}

func TestPrepare_DryRunWritesNothing(t *testing.T) {
	root := fakeRepo(t, `{"builtins":[]}`)

	res, err := Prepare(Options{RepoRoot: root, Version: "1.19.0", DryRun: true})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !res.VersionFileChanged {
		t.Error("dry run should still report the version change it would make")
	}
	if got := read(t, filepath.Join(root, VersionFile)); !strings.Contains(got, `var Version = "1.19.0-dev"`) {
		t.Errorf("dry run modified version.go:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "capabilities", "v1.19.0.json")); !os.IsNotExist(err) {
		t.Errorf("dry run created the snapshot: %v", err)
	}
	if len(res.Generated) != 0 {
		t.Errorf("Generated = %v, want empty for a dry run", res.Generated)
	}
}

func TestPrepare_RerunOverwritesSnapshot(t *testing.T) {
	root := fakeRepo(t, `{"builtins":["first"]}`)
	opts := Options{RepoRoot: root, Version: "1.19.0", SkipGenerate: true}

	if _, err := Prepare(opts); err != nil {
		t.Fatalf("first Prepare: %v", err)
	}

	// Capabilities changed since the first run; a re-run must refresh the
	// snapshot and say so.
	if err := os.WriteFile(filepath.Join(root, CapabilitiesFile), []byte(`{"builtins":["second"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Prepare(opts)
	if err != nil {
		t.Fatalf("second Prepare: %v", err)
	}
	if !res.CapabilitiesOverwritten {
		t.Error("CapabilitiesOverwritten should be true on a re-run")
	}
	if res.VersionFileChanged {
		t.Error("VersionFileChanged should be false when version.go already holds the version")
	}
	if res.PreviousVersion != "1.19.0" {
		t.Errorf("PreviousVersion = %q, want 1.19.0", res.PreviousVersion)
	}
	if got := read(t, filepath.Join(root, "capabilities", "v1.19.0.json")); got != `{"builtins":["second"]}` {
		t.Errorf("snapshot not refreshed: %q", got)
	}
}

func TestPrepare_Errors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(t *testing.T) string
		version string
		wantErr string
	}{
		{
			name:    "invalid version",
			setup:   func(t *testing.T) string { return fakeRepo(t, "{}") },
			version: "not-a-version",
			wantErr: "invalid version",
		},
		{
			name:    "missing version.go",
			setup:   func(t *testing.T) string { return t.TempDir() },
			version: "1.19.0",
			wantErr: "read " + VersionFile,
		},
		{
			name: "missing capabilities.json",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				if err := os.MkdirAll(filepath.Join(root, filepath.Dir(VersionFile)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, VersionFile), []byte(versionGo), 0o644); err != nil {
					t.Fatal(err)
				}
				return root
			},
			version: "1.19.0",
			wantErr: "read " + CapabilitiesFile,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Prepare(Options{RepoRoot: tc.setup(t), Version: tc.version, SkipGenerate: true})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
