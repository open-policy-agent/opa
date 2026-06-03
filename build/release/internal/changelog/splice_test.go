package changelog

import (
	"strings"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"1.19.0", "1.19.0"},
		{"v1.19.0", "1.19.0"},
		{" v1.19.0 ", "1.19.0"},
		{"", ""},
	} {
		if got := NormalizeVersion(tc.in); got != tc.want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSection(t *testing.T) {
	for _, tc := range []struct {
		name, version, body, want string
	}{
		{
			name:    "version and body",
			version: "1.19.0",
			body:    "### Fixes\n\n- ast: fix it\n",
			want:    "## 1.19.0\n\n### Fixes\n\n- ast: fix it\n",
		},
		{
			name:    "no version returns body unchanged",
			version: "",
			body:    "### Fixes\n",
			want:    "### Fixes\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Section(tc.version, tc.body); got != tc.want {
				t.Errorf("Section() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, tc.want)
			}
		})
	}
}

const preamble = `# Change Log

All notable changes to this project will be documented in this file.
`

func TestSplice(t *testing.T) {
	body := "### Fixes\n\n- ast: fix a thing ([#1](u)) authored by @a\n"

	for _, tc := range []struct {
		name     string
		existing string
		version  string
		body     string
		want     string
		wantErr  string
	}{
		{
			name: "unreleased section: generated bullets appended after human prose",
			existing: preamble + `
## Unreleased

### Behavior change: something big

Prose written by a maintainer.

### Backwards Compatibility

- ast: hand-written bullet ([#9](u))

## 1.18.2

Older release.
`,
			version: "1.19.0",
			body:    body,
			want: preamble + `
## 1.19.0

### Behavior change: something big

Prose written by a maintainer.

### Backwards Compatibility

- ast: hand-written bullet ([#9](u))

### Fixes

- ast: fix a thing ([#1](u)) authored by @a

## 1.18.2

Older release.
`,
		},
		{
			name: "unreleased section is the last section",
			existing: preamble + `
## Unreleased

Some prose.
`,
			version: "1.19.0",
			body:    body,
			want: preamble + `
## 1.19.0

Some prose.

### Fixes

- ast: fix a thing ([#1](u)) authored by @a
`,
		},
		{
			name: "unreleased section with no content",
			existing: preamble + `
## Unreleased

## 1.18.2

Older release.
`,
			version: "1.19.0",
			body:    body,
			want: preamble + `
## 1.19.0

### Fixes

- ast: fix a thing ([#1](u)) authored by @a

## 1.18.2

Older release.
`,
		},
		{
			name: "unreleased heading is matched case-insensitively",
			existing: preamble + `
## UNRELEASED

Prose.
`,
			version: "1.19.0",
			body:    body,
			want: preamble + `
## 1.19.0

Prose.

### Fixes

- ast: fix a thing ([#1](u)) authored by @a
`,
		},
		{
			name: "empty body only renames the unreleased heading",
			existing: preamble + `
## Unreleased

Prose.

## 1.18.2
`,
			version: "1.19.0",
			body:    "",
			want: preamble + `
## 1.19.0

Prose.

## 1.18.2
`,
		},
		{
			name: "no unreleased section: new section above the topmost release",
			existing: preamble + `
## 1.18.2

Older release.
`,
			version: "1.19.0",
			body:    body,
			want: preamble + `
## 1.19.0

### Fixes

- ast: fix a thing ([#1](u)) authored by @a

## 1.18.2

Older release.
`,
		},
		{
			name:     "no sections at all: appended after the preamble",
			existing: preamble,
			version:  "1.19.0",
			body:     body,
			want: preamble + `
## 1.19.0

### Fixes

- ast: fix a thing ([#1](u)) authored by @a
`,
		},
		{
			name: "level-3 headings are not mistaken for sections",
			existing: preamble + `
## Unreleased

### Fixes

- existing ([#8](u))
`,
			version: "1.19.0",
			body:    body,
			want: preamble + `
## 1.19.0

### Fixes

- existing ([#8](u))

### Fixes

- ast: fix a thing ([#1](u)) authored by @a
`,
		},
		{
			name: "existing version section is refused",
			existing: preamble + `
## 1.19.0

Already released.
`,
			version: "1.19.0",
			body:    body,
			wantErr: "already present",
		},
		{
			name:     "missing version is an error",
			existing: preamble,
			version:  "",
			body:     body,
			wantErr:  "version is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Splice(tc.existing, tc.version, tc.body)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Splice() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, tc.want)
			}
		})
	}
}

// TestSpliceIsIdempotentOnRerun documents the guard: splicing the same version
// twice fails rather than emitting two sections.
func TestSpliceIsIdempotentOnRerun(t *testing.T) {
	first, err := Splice(preamble+"\n## Unreleased\n\nProse.\n", "1.19.0", "### Fixes\n\n- x ([#1](u))\n")
	if err != nil {
		t.Fatalf("first splice: %v", err)
	}
	if _, err := Splice(first, "1.19.0", "### Fixes\n\n- x ([#1](u))\n"); err == nil {
		t.Fatal("expected second splice to fail")
	}
}
