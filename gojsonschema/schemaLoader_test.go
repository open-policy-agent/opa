// Copyright 2018 johandorland ( https://github.com/johandorland )
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

package gojsonschema

import (
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemaLoaderWithReferenceToAddedSchema(t *testing.T) {
	sl := NewSchemaLoader()
	err := sl.AddSchemas(NewStringLoader(`{
		"$id" : "http://localhost:1234/test1.json",
		"type" : "integer"
		}`))

	assert.Nil(t, err)
	schema, err := sl.Compile(NewReferenceLoader("http://localhost:1234/test1.json"))
	assert.Nil(t, err)
	result, err := schema.Validate(NewStringLoader(`"hello"`))
	assert.Nil(t, err)
	if len(result.Errors()) != 1 || result.Errors()[0].Type() != "invalid_type" {
		t.Errorf("Expected invalid type erorr, instead got %v", result.Errors())
	}
}

func TestCrossReference(t *testing.T) {
	schema1 := NewStringLoader(`{
		"$ref" : "http://localhost:1234/test3.json",
		"definitions" : {
			"foo" : {
				"type" : "integer"
			}
		}
	}`)
	schema2 := NewStringLoader(`{
		"$ref" : "http://localhost:1234/test2.json#/definitions/foo"
	}`)

	sl := NewSchemaLoader()
	err := sl.AddSchema("http://localhost:1234/test2.json", schema1)
	assert.Nil(t, err)
	err = sl.AddSchema("http://localhost:1234/test3.json", schema2)
	assert.Nil(t, err)
	schema, err := sl.Compile(NewStringLoader(`{"$ref" : "http://localhost:1234/test2.json"}`))
	assert.Nil(t, err)
	result, err := schema.Validate(NewStringLoader(`"hello"`))
	assert.Nil(t, err)
	if len(result.Errors()) != 1 || result.Errors()[0].Type() != "invalid_type" {
		t.Errorf("Expected invalid type erorr, instead got %v", result.Errors())
	}
}

// Multiple schemas identifying under the same $id should throw an error
func TestDoubleIDReference(t *testing.T) {
	sl := NewSchemaLoader()
	err := sl.AddSchema("http://localhost:1234/test4.json", NewStringLoader("{}"))
	assert.Nil(t, err)
	err = sl.AddSchemas(NewStringLoader(`{ "$id" : "http://localhost:1234/test4.json"}`))
	assert.NotNil(t, err)
}

func TestCustomMetaSchema(t *testing.T) {

	loader := NewStringLoader(`{
		"$id" : "http://localhost:1234/test5.json",
		"properties" : {
			"multipleOf" : false
		}
	}`)

	// Test a custom metaschema in which we disallow the use of the keyword "multipleOf"
	sl := NewSchemaLoader()
	sl.Validate = true

	err := sl.AddSchemas(loader)
	assert.Nil(t, err)
	_, err = sl.Compile(NewStringLoader(`{
		"$id" : "http://localhost:1234/test6.json",
		"$schema" : "http://localhost:1234/test5.json",
		"type" : "string"
	}`))
	assert.Nil(t, err)

	sl = NewSchemaLoader()
	sl.Validate = true
	err = sl.AddSchemas(loader)
	assert.Nil(t, err)
	_, err = sl.Compile(NewStringLoader(`{
		"$id" : "http://localhost:1234/test7.json",
		"$schema" : "http://localhost:1234/test5.json",
		"multipleOf" : 5
	}`))
	assert.NotNil(t, err)
}

func TestSchemaDetection(t *testing.T) {
	loader := NewStringLoader(`{
		"$schema" : "http://json-schema.org/draft-04/schema#",
		"exclusiveMinimum" : 5
	}`)

	// The schema should produce an error in draft-04 mode
	_, err := NewSchema(loader)
	assert.NotNil(t, err)

	// With schema detection disabled the schema should not produce an error in hybrid mode
	sl := NewSchemaLoader()
	sl.AutoDetect = false

	_, err = sl.Compile(loader)
	assert.Nil(t, err)
}

func TestDraftCrossReferencing(t *testing.T) {

	// Tests the following cross referencing with any combination
	// of autodetection and preset draft version.

	loader1 := NewStringLoader(`{
		"$schema" : "http://json-schema.org/draft-04/schema#",
		"id" : "http://localhost:1234/file.json",
		"$id" : "http://localhost:1234/file.json",
		"exclusiveMinimum" : 5
	}`)
	loader2 := NewStringLoader(`{
		"$schema" : "http://json-schema.org/draft-07/schema#",
		"id" : "http://localhost:1234/main.json",
		"$id" : "http://localhost:1234/main.json",
		"$ref" : "file.json"
	}`)

	for _, b := range []bool{true, false} {
		for _, draft := range []Draft{Draft4, Draft6, Draft7} {
			sl := NewSchemaLoader()
			sl.Draft = draft
			sl.AutoDetect = b

			err := sl.AddSchemas(loader1)
			assert.Nil(t, err)
			_, err = sl.Compile(loader2)

			// It will always fail with autodetection on as "exclusiveMinimum" : 5
			// is only valid since draft-06. With autodetection off it will pass if
			// draft-06 or newer is used

			assert.Equal(t, err == nil, !b && draft >= Draft6)
		}
	}
}

const not_map_interface = "not map interface"

func TestParseSchemaURL_NotMap(t *testing.T) {
	//GIVEN
	sl := NewGoLoader(not_map_interface)
	//WHEN
	_, err := NewSchema(sl)
	//THEN
	require.Error(t, err)
	assert.EqualError(t, err, "schema is invalid")
}
