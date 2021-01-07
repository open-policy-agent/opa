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

// author           xeipuuv
// author-github    https://github.com/xeipuuv
// author-mail      xeipuuv@gmail.com
//
// repository-name  gojsonschema
// repository-desc  An implementation of JSON Schema, based on IETF's draft v4 - Go language.
//
// description      (Unit) Tests for schema validation.
//
// created          16-06-2013

package gojsonschema

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const displayErrorMessages = false

const circularReference = `{
	"type": "object",
	"properties": {
		"games": {
			"type": "array",
			"items": {
				"$ref": "#/definitions/game"
			}
		}
	},
	"definitions": {
		"game": {
			"type": "object",
			"properties": {
				"winner": {
					"$ref": "#/definitions/player"
				},
				"loser": {
					"$ref": "#/definitions/player"
				}
			}
		},
		"player": {
			"type": "object",
			"properties": {
				"user": {
					"$ref": "#/definitions/user"
				},
				"game": {
					"$ref": "#/definitions/game"
				}
			}
		},
		"user": {
			"type": "object",
			"properties": {
				"fullName": {
					"type": "string"
				}
			}
		}
	}
}`

func TestCircularReference(t *testing.T) {
	loader := NewStringLoader(circularReference)
	// call the target function
	_, err := NewSchema(loader)
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
	}
}

// From http://json-schema.org/examples.html
const simpleSchema = `{
  "title": "Example Schema",
  "type": "object",
  "properties": {
    "firstName": {
      "type": "string"
    },
    "lastName": {
      "type": "string"
    },
    "age": {
      "description": "Age in years",
      "type": "integer",
      "minimum": 0
    }
  },
  "required": ["firstName", "lastName"]
}`

func TestLoaders(t *testing.T) {
	// setup reader loader
	reader := bytes.NewBufferString(simpleSchema)
	readerLoader, wrappedReader := NewReaderLoader(reader)

	// drain reader
	by, err := ioutil.ReadAll(wrappedReader)
	assert.Nil(t, err)
	assert.Equal(t, simpleSchema, string(by))

	// setup writer loaders
	writer := &bytes.Buffer{}
	writerLoader, wrappedWriter := NewWriterLoader(writer)

	// fill writer
	n, err := io.WriteString(wrappedWriter, simpleSchema)
	assert.Nil(t, err)
	assert.Equal(t, n, len(simpleSchema))

	loaders := []JSONLoader{
		NewStringLoader(simpleSchema),
		readerLoader,
		writerLoader,
	}

	for _, l := range loaders {
		_, err := NewSchema(l)
		assert.Nil(t, err, "loader: %T", l)
	}
}

const invalidPattern = `{
  "title": "Example Pattern",
  "type": "object",
  "properties": {
    "invalid": {
      "type": "string",
      "pattern": 99999
    }
  }
}`

func TestLoadersWithInvalidPattern(t *testing.T) {
	// setup reader loader
	reader := bytes.NewBufferString(invalidPattern)
	readerLoader, wrappedReader := NewReaderLoader(reader)

	// drain reader
	by, err := ioutil.ReadAll(wrappedReader)
	assert.Nil(t, err)
	assert.Equal(t, invalidPattern, string(by))

	// setup writer loaders
	writer := &bytes.Buffer{}
	writerLoader, wrappedWriter := NewWriterLoader(writer)

	// fill writer
	n, err := io.WriteString(wrappedWriter, invalidPattern)
	assert.Nil(t, err)
	assert.Equal(t, n, len(invalidPattern))

	loaders := []JSONLoader{
		NewStringLoader(invalidPattern),
		readerLoader,
		writerLoader,
	}

	for _, l := range loaders {
		_, err := NewSchema(l)
		assert.NotNil(t, err, "expected error loading invalid pattern: %T", l)
	}
}

const refPropertySchema = `{
	"$id" : "http://localhost/schema.json",
	"properties" : {
		"$id" : {
			"$id": "http://localhost/foo.json"
		},
		"$ref" : {
			"const": {
				"$ref" : "hello.world"
			}
		},
		"const" : {
			"$ref" : "#/definitions/$ref"
		}
	},
	"definitions" : {
		"$ref" : {
			"const": {
				"$ref" : "hello.world"
			}
		}
	},
	"dependencies" : {
		"$ref" : [ "const" ],
		"const" : [ "$ref" ]
	}
}`

func TestRefProperty(t *testing.T) {
	schemaLoader := NewStringLoader(refPropertySchema)
	documentLoader := NewStringLoader(`{
		"$ref" : { "$ref" : "hello.world" },
		"const" : { "$ref" : "hello.world" }
		}`)
	// call the target function
	s, err := NewSchema(schemaLoader)
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
	}
	result, err := s.Validate(documentLoader)
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
	}
	if !result.Valid() {
		for _, err := range result.Errors() {
			fmt.Println(err.String())
		}
		t.Errorf("Got invalid validation result.")
	}
}

func TestFragmentLoader(t *testing.T) {
	wd, err := os.Getwd()

	if err != nil {
		panic(err.Error())
	}

	fileName := filepath.Join(wd, "testdata", "extra", "fragment_schema.json")

	schemaLoader := NewReferenceLoader("file://" + filepath.ToSlash(fileName) + "#/definitions/x")
	schema, err := NewSchema(schemaLoader)

	if err != nil {
		t.Errorf("Encountered error while loading schema: %s", err.Error())
	}

	validDocument := NewStringLoader(`5`)
	invalidDocument := NewStringLoader(`"a"`)

	result, err := schema.Validate(validDocument)

	if assert.Nil(t, err, "Unexpected error while validating document: %T", err) {
		if !result.Valid() {
			t.Errorf("Got invalid validation result.")
		}
	}

	result, err = schema.Validate(invalidDocument)

	if assert.Nil(t, err, "Unexpected error while validating document: %T", err) {
		if len(result.Errors()) != 1 || result.Errors()[0].Type() != "invalid_type" {
			t.Errorf("Got invalid validation result.")
		}
	}
}

func TestFileWithSpace(t *testing.T) {
	wd, err := os.Getwd()

	if err != nil {
		panic(err.Error())
	}

	fileName := filepath.Join(wd, "testdata", "extra", "file with space.json")
	loader := NewReferenceLoader("file://" + filepath.ToSlash(fileName))

	json, err := loader.LoadJSON()

	assert.Nil(t, err, "Unexpected error when trying to load a filepath containing a space")
	assert.Equal(t, map[string]interface{}{"foo": true}, json, "Contents of the file do not match")
}

func TestAdditionalPropertiesErrorMessage(t *testing.T) {
	schema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "Device": {
      "type": "object",
      "additionalProperties": {
        "type": "string"
      }
    }
  }
}`
	text := `{
		"Device":{
			"Color" : true
		}
	}`
	loader := NewBytesLoader([]byte(schema))
	result, err := Validate(loader, NewBytesLoader([]byte(text)))
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Errors()) != 1 {
		t.Fatal("Expected 1 error but got", len(result.Errors()))
	}

	expected := "Device.Color: Invalid type. Expected: string, given: boolean"
	actual := result.Errors()[0].String()
	if actual != expected {
		t.Fatalf("Expected '%s' but got '%s'", expected, actual)
	}
}

// Inspired by http://json-schema.org/latest/json-schema-core.html#rfc.section.8.2.3
const locationIndependentSchema = `{
  "definitions": {
    "A": {
      "$id": "#foo"
    },
    "B": {
      "$id": "http://example.com/other.json",
      "definitions": {
        "X": {
          "$id": "#bar",
          "allOf": [false]
        },
        "Y": {
          "$id": "t/inner.json"
        }
      }
    },
    "C": {
			"$id" : "#frag",
      "$ref": "http://example.com/other.json#bar"
    }
  },
  "$ref": "#frag"
}`

func TestLocationIndependentIdentifier(t *testing.T) {
	schemaLoader := NewStringLoader(locationIndependentSchema)
	documentLoader := NewStringLoader(`{}`)

	s, err := NewSchema(schemaLoader)
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
	}

	result, err := s.Validate(documentLoader)
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
	}

	if len(result.Errors()) != 2 || result.Errors()[0].Type() != "false" || result.Errors()[1].Type() != "number_all_of" {
		t.Errorf("Got invalid validation result.")
	}
}

const incorrectRefSchema = `{
  "$ref" : "#/fail"
}`

func TestIncorrectRef(t *testing.T) {

	schemaLoader := NewStringLoader(incorrectRefSchema)
	s, err := NewSchema(schemaLoader)

	assert.Nil(t, s)
	assert.Equal(t, "Object has no key 'fail'", err.Error())
}
