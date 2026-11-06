package changelog

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestExtractSection(t *testing.T) {
	release := preamble + `
## 1.22.2

This is a bugfix release.

### Fixes

- ast: fix it ([#1](u))

## 1.22.1

Older release.
`
	got, err := ExtractSection(release, "1.22.2")
	if err != nil {
		t.Fatal(err)
	}
	want := "## 1.22.2\n\nThis is a bugfix release.\n\n### Fixes\n\n- ast: fix it ([#1](u))\n"
	if got != want {
		t.Errorf("ExtractSection() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	if _, err := ExtractSection(release, "1.22.3"); err == nil {
		t.Error("expected an error for a missing section")
	}
}

func TestInsertSection(t *testing.T) {
	section := "## 1.21.5\n\nPatch.\n"

	for _, tc := range []struct {
		name, existing, want string
	}{
		{
			name: "below unreleased and newer minors",
			existing: preamble + `
## Unreleased

Prose.

## 1.22.0

Minor.

## 1.21.4

Older patch.
`,
			want: preamble + `
## Unreleased

Prose.

## 1.22.0

Minor.

## 1.21.5

Patch.

## 1.21.4

Older patch.
`,
		},
		{
			name: "non-release headings are skipped",
			existing: preamble + `
## Compiler

## 1.2.0
`,
			want: preamble + `
## Compiler

## 1.21.5

Patch.

## 1.2.0
`,
		},
		{
			name: "no older release: appended",
			existing: preamble + `
## 1.22.0

Minor.
`,
			want: preamble + `
## 1.22.0

Minor.

## 1.21.5

Patch.
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := InsertSection(tc.existing, "1.21.5", section)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("InsertSection() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, tc.want)
			}
		})
	}
}

func TestDropReleased(t *testing.T) {
	section := `## 1.21.5

### Security fix ([GHSA](u))

The body.

#### Details

More.

### Renamed fix

Same body.
`
	existing := preamble + `
## Unreleased

Prose above the subsections.

### Security fix (TODO-LINK)

The body.

#### Details

More.

### A new feature

Unrelated.

### Renamed fix

Edited body.

## 1.21.4

### Renamed fix

Same body.
`
	want := preamble + `
## Unreleased

Prose above the subsections.

### A new feature

Unrelated.

## 1.21.4

### Renamed fix

Same body.
`
	got, dropped := DropReleased(existing, section)
	if got != want {
		t.Errorf("DropReleased() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if want := []string{"Security fix (TODO-LINK)", "Renamed fix"}; !slices.Equal(dropped, want) {
		t.Errorf("dropped = %q, want %q", dropped, want)
	}

	if got, dropped := DropReleased(want, section); got != want || dropped != nil {
		t.Errorf("nothing to drop: got %q, dropped %q", got, dropped)
	}
}

func TestInsertSection_Errors(t *testing.T) {
	_, err := InsertSection(preamble+"\n## 1.21.5\n", "1.21.5", "## 1.21.5\n")
	if !errors.Is(err, ErrSectionExists) {
		t.Errorf("err = %v, want ErrSectionExists", err)
	}

	_, err = InsertSection(preamble, "1.21.5-rc.1", "## 1.21.5-rc.1\n")
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Errorf("err = %v, want invalid release version", err)
	}
}
