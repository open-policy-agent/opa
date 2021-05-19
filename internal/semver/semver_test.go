// Copyright 2013-2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// vendored from: https://github.com/coreos/go-semver/tree/e214231b295a8ea9479f11b70b35d5acf3556d9b/semver
package semver

import (
	"testing"
)

type fixture struct {
	GreaterVersion string
	LesserVersion  string
}

var fixtures = []fixture{
	{"0.0.0", "0.0.0-foo"},
	{"0.0.1", "0.0.0"},
	{"1.0.0", "0.9.9"},
	{"0.10.0", "0.9.0"},
	{"0.99.0", "0.10.0"},
	{"2.0.0", "1.2.3"},
	{"0.0.0", "0.0.0-foo"},
	{"0.0.1", "0.0.0"},
	{"1.0.0", "0.9.9"},
	{"0.10.0", "0.9.0"},
	{"0.99.0", "0.10.0"},
	{"2.0.0", "1.2.3"},
	{"0.0.0", "0.0.0-foo"},
	{"0.0.1", "0.0.0"},
	{"1.0.0", "0.9.9"},
	{"0.10.0", "0.9.0"},
	{"0.99.0", "0.10.0"},
	{"2.0.0", "1.2.3"},
	{"1.2.3", "1.2.3-asdf"},
	{"1.2.3", "1.2.3-4"},
	{"1.2.3", "1.2.3-4-foo"},
	{"1.2.3-5-foo", "1.2.3-5"},
	{"1.2.3-5", "1.2.3-4"},
	{"1.2.3-5-foo", "1.2.3-5-Foo"},
	{"3.0.0", "2.7.2+asdf"},
	{"3.0.0+foobar", "2.7.2"},
	{"1.2.3-a.10", "1.2.3-a.5"},
	{"1.2.3-a.b", "1.2.3-a.5"},
	{"1.2.3-a.b", "1.2.3-a"},
	{"1.2.3-a.b.c.10.d.5", "1.2.3-a.b.c.5.d.100"},
	{"1.0.0", "1.0.0-rc.1"},
	{"1.0.0-rc.2", "1.0.0-rc.1"},
	{"1.0.0-rc.1", "1.0.0-beta.11"},
	{"1.0.0-beta.11", "1.0.0-beta.2"},
	{"1.0.0-beta.2", "1.0.0-beta"},
	{"1.0.0-beta", "1.0.0-alpha.beta"},
	{"1.0.0-alpha.beta", "1.0.0-alpha.1"},
	{"1.0.0-alpha.1", "1.0.0-alpha"},
	{"1.2.3-rc.1-1-1hash", "1.2.3-rc.2"},
}

func TestCompare(t *testing.T) {
	for _, v := range fixtures {
		gt, err := NewVersion(v.GreaterVersion)
		if err != nil {
			t.Error(err)
		}

		lt, err := NewVersion(v.LesserVersion)
		if err != nil {
			t.Error(err)
		}

		if gt.Compare(*lt) <= 0 {
			t.Errorf("%s should be greater than %s", gt, lt)
		}
		if lt.Compare(*gt) > 0 {
			t.Errorf("%s should not be greater than %s", lt, gt)
		}
	}
}

func TestBadInput(t *testing.T) {
	bad := []string{
		"1.2",
		"1.2.3x",
		"0x1.3.4",
		"-1.2.3",
		"1.2.3.4",
		"0.88.0-11_e4e5dcabb",
		"0.88.0+11_e4e5dcabb",
	}
	for _, b := range bad {
		if _, err := NewVersion(b); err == nil {
			t.Error("Improperly accepted value: ", b)
		}
	}
}
