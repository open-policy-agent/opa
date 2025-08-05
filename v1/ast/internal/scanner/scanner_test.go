// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package scanner

import (
	"bytes"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast/internal/tokens"
)

func TestPositions(t *testing.T) {
	tests := []struct {
		note       string
		input      string
		wantOffset int
		wantEnd    int
	}{
		{
			note:       "symbol",
			input:      "(",
			wantOffset: 0,
			wantEnd:    1,
		},
		{
			note:       "ident",
			input:      "foo",
			wantOffset: 0,
			wantEnd:    3,
		},
		{
			note:       "number",
			input:      "100",
			wantOffset: 0,
			wantEnd:    3,
		},
		{
			note:       "string",
			input:      `"foo"`,
			wantOffset: 0,
			wantEnd:    5,
		},
		{
			note:       "string - wide char",
			input:      `"foo÷"`,
			wantOffset: 0,
			wantEnd:    7,
		},
		{
			note:       "comment",
			input:      `# foo`,
			wantOffset: 0,
			wantEnd:    5,
		},
		{
			note:       "newline",
			input:      "foo\n",
			wantOffset: 0,
			wantEnd:    3,
		},
		{
			note:       "invalid number",
			input:      "0xDEADBEEF",
			wantOffset: 0,
			wantEnd:    10,
		},
		{
			note:       "invalid identifier",
			input:      "0.1e12a1b2c3d",
			wantOffset: 0,
			wantEnd:    13,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			s, err := New(bytes.NewBufferString(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			_, pos, _, _ := s.Scan()
			if pos.Offset != tc.wantOffset {
				t.Fatalf("want offset %d but got %d", tc.wantOffset, pos.Offset)
			}
			if pos.End != tc.wantEnd {
				t.Fatalf("want end %d but got %d", tc.wantEnd, pos.End)
			}
		})
	}
}

func TestLiterals(t *testing.T) {

	tests := []struct {
		note       string
		input      string
		wantRow    int
		wantOffset int
		wantTok    tokens.Token
		wantLit    string
	}{
		{
			note:       "ascii chars",
			input:      `"hello world"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.String,
			wantLit:    `"hello world"`,
		},
		{
			note:       "wide chars",
			input:      `"¡¡¡foo, bar!!!"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.String,
			wantLit:    `"¡¡¡foo, bar!!!"`,
		},
		{
			note:       "raw strings",
			input:      "`foo`",
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.String,
			wantLit:    "`foo`",
		},
		{
			note:       "raw strings - wide chars",
			input:      "`¡¡¡foo, bar!!!`",
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.String,
			wantLit:    "`¡¡¡foo, bar!!!`",
		},
		{
			note:       "comments",
			input:      "# foo",
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.Comment,
			wantLit:    "# foo",
		},
		{
			note:       "comments - wide chars",
			input:      "#¡foo",
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.Comment,
			wantLit:    "#¡foo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			s, err := New(bytes.NewBufferString(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			tok, pos, lit, errs := s.Scan()
			if pos.Row != tc.wantRow {
				t.Errorf("Expected row %d but got %d", tc.wantRow, pos.Row)
			}
			if pos.Offset != tc.wantOffset {
				t.Errorf("Expected offset %d but got %d", tc.wantOffset, pos.Offset)
			}
			if tok != tc.wantTok {
				t.Errorf("Expected token %v but got %v", tc.wantTok, tok)
			}
			if lit != tc.wantLit {
				t.Errorf("Expected literal %v but got %v", tc.wantLit, lit)
			}
			if len(errs) > 0 {
				t.Fatal("Unexpected error(s):", errs)
			}
		})
	}

}

func TestTemplateStrings(t *testing.T) {
	tests := []struct {
		note       string
		input      string
		opts       []ScanOption
		offset     int
		wantRow    int
		wantOffset int
		wantTok    tokens.Token
		wantLit    string
	}{
		{
			note:       "no template expressions",
			input:      `$"foo bar"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringEnd,
			wantLit:    `"foo bar"`,
		},
		{
			note:       "with template expression",
			input:      `$"foo {1 + 2} bar"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringPart,
			wantLit:    `"foo {`,
		},
		{
			note:       "with template expression, continued",
			input:      `} bar"`,
			opts:       []ScanOption{ContinueTemplateString()},
			offset:     2, // the closing brace would have already been scanned as part of the template expression
			wantRow:    1,
			wantOffset: 1,
			wantTok:    tokens.TemplateStringEnd,
			wantLit:    `} bar"`,
		},
		{
			note:       "with multiple template expressions, continued",
			input:      `} bar { 1 + 2 } baz"`,
			opts:       []ScanOption{ContinueTemplateString()},
			offset:     2, // the closing brace would have already been scanned as part of the template expression
			wantRow:    1,
			wantOffset: 1,
			wantTok:    tokens.TemplateStringPart,
			wantLit:    `} bar {`,
		},
		{
			note:       "with escaped template expression, leading",
			input:      `$"\{1 + 2} foo"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringEnd,
			wantLit:    `"{1 + 2} foo"`,
		},
		{
			note:       "with escaped template expression, leading, both braces",
			input:      `$"\{1 + 2\} foo"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringEnd,
			wantLit:    `"{1 + 2} foo"`,
		},
		{
			note:       "with escaped template expression, middle",
			input:      `$"foo \{1 + 2} bar"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringEnd,
			wantLit:    `"foo {1 + 2} bar"`,
		},
		{
			note:       "with escaped template expression, middle, both braces",
			input:      `$"foo \{1 + 2\} bar"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringEnd,
			wantLit:    `"foo {1 + 2} bar"`,
		},
		{
			note:       "with escaped template expression, trailing",
			input:      `$"foo \{1 + 2}"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringEnd,
			wantLit:    `"foo {1 + 2}"`,
		},
		{
			note:       "with escaped template expression, trailing, both braces",
			input:      `$"foo \{1 + 2\}"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringEnd,
			wantLit:    `"foo {1 + 2}"`,
		},
		{
			note:       "with escaped template expression, containing actual template expression",
			input:      `$"foo \{{1} + 2}"`,
			wantRow:    1,
			wantOffset: 0,
			wantTok:    tokens.TemplateStringPart,
			wantLit:    `"foo {{`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			s, err := New(bytes.NewBufferString(tc.input))
			if tc.offset != 0 {
				s.offset = tc.offset
			}
			if err != nil {
				t.Fatal(err)
			}
			tok, pos, lit, errs := s.Scan(tc.opts...)
			if pos.Row != tc.wantRow {
				t.Errorf("Expected row %d but got %d", tc.wantRow, pos.Row)
			}
			if pos.Offset != tc.wantOffset {
				t.Errorf("Expected offset %d but got %d", tc.wantOffset, pos.Offset)
			}
			if tok != tc.wantTok {
				t.Errorf("Expected token %v but got %v", tc.wantTok, tok)
			}
			if lit != tc.wantLit {
				t.Errorf("Expected literal %v but got %v", tc.wantLit, lit)
			}
			if len(errs) > 0 {
				t.Fatal("Unexpected error(s):", errs)
			}
		})
	}
}

func TestIllegalTokens(t *testing.T) {

	tests := []struct {
		input   string
		wantErr bool
	}{
		{input: `墳`},
		{input: `0e`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			s, err := New(bytes.NewBufferString(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			tok, _, _, errs := s.Scan()
			if !tc.wantErr && tok != tokens.Illegal {
				t.Fatalf("expected illegal token on %q but got %v", tc.input, tok)
			} else if tc.wantErr && len(errs) == 0 {
				t.Fatalf("expected errors on %q but got %v", tc.input, tok)
			}
		})
	}
}
