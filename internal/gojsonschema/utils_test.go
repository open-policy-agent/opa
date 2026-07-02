// Copyright 2015 xeipuuv ( https://github.com/xeipuuv )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author           janmentzel
// author-github    https://github.com/janmentzel
// author-mail      ? ( forward to xeipuuv@gmail.com )
//
// repository-name  gojsonschema
// repository-desc  An implementation of JSON Schema, based on IETF's draft v4 - Go language.
//
// description     (Unit) Tests for utils ( Float / Integer conversion ).
//
// created          08-08-2013

package gojsonschema

import (
	"encoding/json"
	"testing"
)

func TestCheckJsonNumber(t *testing.T) {
	var testCases = []struct {
		isInt bool
		value json.Number
	}{
		{true, "0"},
		{true, "2147483647"},
		{true, "-2147483648"},
		{true, "9223372036854775807"},
		{true, "-9223372036854775808"},
		{true, "1.0e+2"},
		{true, "1.0e+10"},
		{true, "-1.0e+2"},
		{true, "-1.0e+10"},
		{false, "1.0e-2"},
		{false, "number"},
		{false, "123number"},
	}

	for _, testCase := range testCases {
		if exp, got := testCase.isInt, checkJSONInteger(testCase.value); exp != got {
			t.Errorf("Expected %v, got %v for %v", exp, got, testCase.value)
		}
	}

}
