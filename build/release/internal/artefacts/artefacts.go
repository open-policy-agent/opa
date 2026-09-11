// Package artefacts bumps the version, snapshots capabilities and regenerates
// the derived files, leaving everything in the working tree for review. Nothing
// here touches git.
package artefacts

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	VersionFile      = "v1/version/version.go"
	CapabilitiesFile = "capabilities.json"
	CapabilitiesDir  = "capabilities"
	MetadataFile     = "builtin_metadata.json"
	VersionIndexFile = "v1/ast/version_index.json"
)

// Options configures Prepare.
type Options struct {
	RepoRoot string
	Version  string
	DryRun   bool
	// SkipGenerate leaves builtin_metadata.json and version_index.json stale.
	SkipGenerate bool
	Logf         func(format string, args ...any)
}

// Result describes what Prepare did, or under DryRun would do.
type Result struct {
	PreviousVersion    string
	VersionFileChanged bool
	CapabilitiesPath   string
	// CapabilitiesOverwritten flags that a tracked file changed, the normal case
	// on a re-run.
	CapabilitiesOverwritten bool
	Generated               []string
}

var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.\-]+)?$`)

// ParseVersion strips a leading "v" and validates the rest.
func ParseVersion(s string) (string, error) {
	v := strings.TrimPrefix(strings.TrimSpace(s), "v")
	if v == "" {
		return "", errors.New("version is required")
	}
	if !versionPattern.MatchString(v) {
		return "", fmt.Errorf("invalid version %q, expected X.Y.Z (optionally with a -prerelease suffix)", s)
	}
	return v, nil
}

var versionAssign = regexp.MustCompile(`(?m)^(var Version = ")([^"]*)(")$`)

// SetVersion requires exactly one assignment, so an unexpected version.go fails
// rather than being silently mangled.
func SetVersion(content, version string) (string, string, error) {
	matches := versionAssign.FindAllStringSubmatchIndex(content, -1)
	switch len(matches) {
	case 1:
	case 0:
		return "", "", fmt.Errorf(`no 'var Version = "..."' assignment found in %s`, VersionFile)
	default:
		return "", "", fmt.Errorf(`found %d 'var Version = "..."' assignments in %s, expected exactly one`, len(matches), VersionFile)
	}

	m := matches[0]
	previous := content[m[4]:m[5]]
	updated := content[:m[4]] + version + content[m[5]:]
	return updated, previous, nil
}

// SnapshotName returns e.g. "capabilities/v1.19.0.json".
func SnapshotName(version string) string {
	return filepath.ToSlash(filepath.Join(CapabilitiesDir, "v"+version+".json"))
}

// Prepare bumps the version, regenerates capabilities.json, snapshots it, then
// regenerates the rest.
//
// The order is forced: capabilities.json must be current before it is
// snapshotted, and the version index reads the go:embed'd capabilities/
// directory, so it only sees the new snapshot once that file exists on disk.
func Prepare(opts Options) (*Result, error) {
	version, err := ParseVersion(opts.Version)
	if err != nil {
		return nil, err
	}
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	res := &Result{CapabilitiesPath: SnapshotName(version)}

	previous, changed, err := BumpVersion(opts.RepoRoot, version, opts.DryRun, logf)
	if err != nil {
		return nil, err
	}
	res.PreviousVersion, res.VersionFileChanged = previous, changed

	if !opts.SkipGenerate && !opts.DryRun {
		if err := runGenerator(opts.RepoRoot, genCapabilities, logf); err != nil {
			return nil, err
		}
		res.Generated = append(res.Generated, genCapabilities.out)
	}

	if err := snapshotCapabilities(opts, res, logf); err != nil {
		return nil, err
	}

	if opts.SkipGenerate {
		logf("skipping code generation (--skip-generate); %s and %s are now stale", MetadataFile, VersionIndexFile)
		return res, nil
	}
	if opts.DryRun {
		logf("would regenerate %s, %s and %s", CapabilitiesFile, MetadataFile, VersionIndexFile)
		return res, nil
	}
	for _, g := range []generator{genBuiltinMetadata, genVersionIndex} {
		if err := runGenerator(opts.RepoRoot, g, logf); err != nil {
			return nil, err
		}
		res.Generated = append(res.Generated, g.out)
	}
	return res, nil
}

// BumpVersion writes version into version.go, returning the version it replaced
// and whether the file changed.
func BumpVersion(root, version string, dryRun bool, logf func(string, ...any)) (string, bool, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	path := filepath.Join(root, VersionFile)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", VersionFile, err)
	}
	updated, previous, err := SetVersion(string(content), version)
	if err != nil {
		return "", false, err
	}
	changed := previous != version

	switch {
	case !changed:
		logf("%s already reads Version = %q", VersionFile, version)
		return previous, false, nil
	case dryRun:
		logf("would set %s: Version %q -> %q", VersionFile, previous, version)
		return previous, true, nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", false, fmt.Errorf("write %s: %w", VersionFile, err)
	}
	logf("%s: Version %q -> %q", VersionFile, previous, version)
	return previous, true, nil
}

func snapshotCapabilities(opts Options, res *Result, logf func(string, ...any)) error {
	src := filepath.Join(opts.RepoRoot, CapabilitiesFile)
	dst := filepath.Join(opts.RepoRoot, filepath.FromSlash(res.CapabilitiesPath))

	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", CapabilitiesFile, err)
	}
	if _, err := os.Stat(dst); err == nil {
		res.CapabilitiesOverwritten = true
	}

	if opts.DryRun {
		verb := "write"
		if res.CapabilitiesOverwritten {
			verb = "overwrite"
		}
		logf("would %s %s (%d bytes) from %s", verb, res.CapabilitiesPath, len(content), CapabilitiesFile)
		return nil
	}
	verb := "wrote"
	if res.CapabilitiesOverwritten {
		verb = "overwrote"
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", res.CapabilitiesPath, err)
	}
	logf("%s %s (%d bytes) from %s", verb, res.CapabilitiesPath, len(content), CapabilitiesFile)
	return nil
}

type generator struct {
	script string
	out    string
}

// Deliberately not `make generate`: that depends on wasm-lib-build, which
// rebuilds opa.wasm through docker and would put an unrelated binary in the
// release diff. These are the same commands main.go's //go:generate lines run.
var (
	genCapabilities    = generator{"internal/cmd/genopacapabilities/main.go", CapabilitiesFile}
	genBuiltinMetadata = generator{"internal/cmd/genbuiltinmetadata/main.go", MetadataFile}
	genVersionIndex    = generator{"internal/cmd/genversionindex/main.go", VersionIndexFile}
)

// runGenerator mirrors build/gen-run-go.sh, clearing GOOS/GOARCH/CC so a
// cross-compilation environment doesn't leak into a host-run generator.
func runGenerator(root string, g generator, logf func(format string, args ...any)) error {
	logf("generating %s", g.out)
	cmd := exec.Command("go", "run", "-tags", "generate", g.script, g.out)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=", "GOARCH=", "CC=")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("generate %s (go run %s): %w", g.out, g.script, err)
	}
	return nil
}
