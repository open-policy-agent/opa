// Copyright 2026 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/v1/util"
)

func TestRelatedResourceAnnotationJSONRoundTrip(t *testing.T) {
	for _, ref := range []string{
		"https://example.com/privacy?purpose=analytics#retention",
		"https://example.com/policy%20reference",
		"mailto:privacy@example.com",
		"../policies/retention",
		"",
	} {
		t.Run(ref, func(t *testing.T) {
			want := &RelatedResourceAnnotation{Ref: mustParseURL(ref), Description: "Policy reference"}
			bs, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var got RelatedResourceAnnotation
			if err := util.UnmarshalJSON(bs, &got); err != nil {
				t.Fatal(err)
			}
			if got.Compare(want) != 0 {
				t.Fatalf("expected %v, got %v", want, &got)
			}
		})
	}
}

func TestRelatedResourceAnnotationJSONInvalidReference(t *testing.T) {
	for _, input := range []string{
		`{"ref":"https://example.com/%zz"}`,
		`{"ref":"https://[invalid"}`,
		`{"ref":123}`,
		`{"ref":[]}`,
	} {
		t.Run(input, func(t *testing.T) {
			var got RelatedResourceAnnotation
			if err := util.UnmarshalJSON([]byte(input), &got); err == nil {
				t.Fatalf("expected an error for %s", input)
			}
		})
	}
}

func TestRelatedResourceAnnotationJSONLegacyURL(t *testing.T) {
	var got RelatedResourceAnnotation
	if err := util.UnmarshalJSON([]byte(`{"ref":{"Scheme":"https","Host":"example.com","Path":"/policy"},"description":"Policy reference"}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Ref.String() != "https://example.com/policy" || got.Description != "Policy reference" {
		t.Fatalf("unexpected related resource: %v", &got)
	}
}
