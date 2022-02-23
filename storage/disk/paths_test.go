// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"testing"

	"github.com/open-policy-agent/opa/storage"
)

func TestIsDisjoint(t *testing.T) {
	paths := func(ps ...string) pathSet {
		ret := make([]storage.Path, len(ps))
		for i := range ps {
			ret[i] = storage.MustParsePath(ps[i])
		}
		return ret
	}

	for _, tc := range []struct {
		note       string
		ps         pathSet
		overlapped bool
	}{
		{
			note: "simple disjoint",
			ps:   paths("/foo", "/bar", "/baz"),
		},
		{
			note:       "simple overlapping",
			ps:         paths("/foo", "/foo/bar"),
			overlapped: true,
		},
		{
			note:       "three overlapping",
			ps:         paths("/fox", "/foo/bar", "/foo"),
			overlapped: true,
		},
		{
			note:       "wildcard overlapping, last",
			ps:         paths("/foo", "/foo/*"),
			overlapped: true,
		},
		{
			note:       "wildcard overlapping, middle",
			ps:         paths("/foo/bar/baz", "/foo/*/baz"),
			overlapped: true,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			act := tc.ps.IsDisjoint()
			if !tc.overlapped != act {
				t.Errorf("path set: %v, disjoint == %v, expected %v", tc.ps, act, !tc.overlapped)
			}
		})
	}
}
