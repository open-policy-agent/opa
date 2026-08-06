package changelog

import "testing"

func TestFilterDependencies_Rules(t *testing.T) {
	cases := []struct {
		name       string
		entry      Entry
		wantKept   bool
		wantAuthor string // post-filter; only meaningful when kept
		wantAction FilterAction
	}{
		{
			name: "non-dep entry passes through unchanged",
			entry: Entry{
				SHA: "1", Subject: "Allow $ref", Area: "ast",
				AuthorLogin: "deeglaze",
			},
			wantKept:   true,
			wantAuthor: "deeglaze",
			wantAction: ActionKept,
		},
		{
			name: "human-authored dep keeps attribution (user-assisted)",
			entry: Entry{
				SHA: "2", Subject: "bump the dependencies group across 2 directories",
				Area: "build(deps)", IsDependency: true, AuthorLogin: "anderseknert",
				Files: []string{"go.mod", "go.sum"},
			},
			wantKept:   true,
			wantAuthor: "anderseknert",
			wantAction: ActionKept,
		},
		{
			name: "dependabot dep with go.mod changes is dropped (covered by go.mod synthesis)",
			entry: Entry{
				SHA: "3", Subject: "Bump foo from 1 to 2", Area: "build(deps)",
				AuthorLogin: "dependabot[bot]", IsDependency: true,
				Files: []string{"go.mod", "go.sum"},
			},
			wantKept:   false,
			wantAction: ActionDropped,
		},
		{
			name: "e2e-only dep is dropped (regression: build(deps) prefix masked path)",
			entry: Entry{
				SHA: "4", Subject: "bump the e2e-prisma group with 3 updates",
				Area: "build(deps)", IsDependency: true, AuthorLogin: "dependabot[bot]",
				Files: []string{"e2e/api/compile/prisma/package.json", "e2e/api/compile/prisma/package-lock.json"},
			},
			wantKept:   false,
			wantAction: ActionDropped,
		},
		{
			name: "docs-only dep is dropped",
			entry: Entry{
				SHA: "5", Subject: "bump mermaid from 11.14.0 to 11.15.0 in /docs",
				Area: "build(deps)", IsDependency: true, AuthorLogin: "dependabot[bot]",
				Files: []string{"docs/package.json", "docs/package-lock.json"},
			},
			wantKept:   false,
			wantAction: ActionDropped,
		},
		{
			name: ".github-only dep is dropped",
			entry: Entry{
				SHA: "6", Subject: "bump the gha-dependencies group with 2 updates",
				Area: "build(deps)", IsDependency: true, AuthorLogin: "dependabot[bot]",
				Files: []string{".github/workflows/ci.yml", ".github/workflows/release.yml"},
			},
			wantKept:   false,
			wantAction: ActionDropped,
		},
		{
			name: "mixed dep (go.mod + workflow tweak) by dependabot is dropped",
			entry: Entry{
				SHA: "7", Subject: "Bump foo", Area: "build(deps)",
				IsDependency: true, AuthorLogin: "dependabot[bot]",
				Files: []string{"go.mod", "go.sum", ".github/workflows/ci.yml"},
			},
			wantKept:   false,
			wantAction: ActionDropped,
		},
		{
			name: "dependabot dep with no path data is dropped",
			entry: Entry{
				SHA: "8", Subject: "Bump bar", Area: "build(deps)",
				IsDependency: true, AuthorLogin: "dependabot[bot]",
				Files: nil,
			},
			wantKept:   false,
			wantAction: ActionDropped,
		},
		{
			name: "human-authored dep with no path data is kept",
			entry: Entry{
				SHA: "9", Subject: "Bump baz", Area: "build(deps)",
				IsDependency: true, AuthorLogin: "anderseknert",
				Files: nil,
			},
			wantKept:   true,
			wantAuthor: "anderseknert",
			wantAction: ActionKept,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kept, logs := FilterDependencies([]Entry{tc.entry})
			if len(logs) != 1 {
				t.Fatalf("expected 1 log, got %d", len(logs))
			}
			if logs[0].Action != tc.wantAction {
				t.Errorf("action: got %q, want %q (reason=%q)", logs[0].Action, tc.wantAction, logs[0].Reason)
			}
			if tc.wantKept {
				if len(kept) != 1 {
					t.Fatalf("expected entry kept, got %d entries", len(kept))
				}
				if kept[0].AuthorLogin != tc.wantAuthor {
					t.Errorf("AuthorLogin: got %q, want %q", kept[0].AuthorLogin, tc.wantAuthor)
				}
			} else if len(kept) != 0 {
				t.Errorf("expected entry dropped, got %d entries", len(kept))
			}
		})
	}
}

func TestFilterDependencies_PreservesOrder(t *testing.T) {
	// Mix kept and dropped to confirm log order matches input order, while
	// dropped entries are absent from the kept slice.
	entries := []Entry{
		{SHA: "A", Subject: "ast: x"},
		{SHA: "B", Subject: "Bump foo", Area: "build(deps)", IsDependency: true, AuthorLogin: "dependabot[bot]", Files: []string{"go.mod"}},
		{SHA: "C", Subject: "Bump trivy", Area: "build(deps)", IsDependency: true, AuthorLogin: "dependabot[bot]", Files: []string{".github/workflows/ci.yml"}},
		{SHA: "D", Subject: "server: y"},
	}
	kept, logs := FilterDependencies(entries)

	wantLogActions := []FilterAction{ActionKept, ActionDropped, ActionDropped, ActionKept}
	for i, want := range wantLogActions {
		if logs[i].Action != want {
			t.Errorf("logs[%d].Action: got %q, want %q (sha=%s)", i, logs[i].Action, want, logs[i].SHA)
		}
	}
	wantKeptSHAs := []string{"A", "D"}
	if len(kept) != len(wantKeptSHAs) {
		t.Fatalf("kept count: got %d, want %d", len(kept), len(wantKeptSHAs))
	}
	for i, want := range wantKeptSHAs {
		if kept[i].SHA != want {
			t.Errorf("kept[%d].SHA: got %s, want %s", i, kept[i].SHA, want)
		}
	}
}

func TestFilterReleaseMechanics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subject string
		want    bool // true == dropped
	}{
		{name: "release tag commit", subject: "Release v1.17.0", want: true},
		{name: "release without v", subject: "Release 1.17.0", want: true},
		{name: "prepare development", subject: "Prepare v1.17.0 development", want: true},
		{name: "integrate patch release", subject: "Integrate 1.16.2 patch release", want: true},
		{name: "case insensitive", subject: "release v1.17.0", want: true},
		{name: "trailing whitespace tolerated", subject: "Release v1.17.0  ", want: true},
		{name: "release notes doc is kept", subject: "Release notes for the docs site", want: false},
		{name: "real change mentioning a version is kept", subject: "Fix panic introduced in v1.17.0", want: false},
		{name: "prepare something else is kept", subject: "Prepare v1.17.0 development docs overhaul", want: false},
		{name: "ordinary change is kept", subject: "Add `inmem.NewFromASTObject`", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kept, logs := FilterReleaseMechanics([]Entry{{SHA: "abc", Subject: tc.subject}})
			dropped := len(kept) == 0
			if dropped != tc.want {
				t.Fatalf("dropped = %v, want %v", dropped, tc.want)
			}
			if tc.want {
				if len(logs) != 1 || logs[0].Action != ActionDropped {
					t.Fatalf("expected one drop log, got %+v", logs)
				}
				if logs[0].Reason != "release mechanics" {
					t.Errorf("reason = %q", logs[0].Reason)
				}
			} else if len(logs) != 0 {
				t.Errorf("expected no logs for a kept entry, got %+v", logs)
			}
		})
	}
}
