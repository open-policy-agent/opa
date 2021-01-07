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
// description      Defines the structure of a sub-SubSchema.
//                  A sub-SubSchema can contain other sub-schemas.
//
// created          27-02-2013

package gojsonschema

import (
	"math/big"
	"regexp"

	"github.com/xeipuuv/gojsonreference"
)

// Constants
const (
	KEY_SCHEMA                = "$schema"
	KEY_ID                    = "id"
	KEY_ID_NEW                = "$id"
	KEY_REF                   = "$ref"
	KEY_TITLE                 = "title"
	KEY_DESCRIPTION           = "description"
	KEY_TYPE                  = "type"
	KEY_ITEMS                 = "items"
	KEY_ADDITIONAL_ITEMS      = "additionalItems"
	KEY_PROPERTIES            = "properties"
	KEY_PATTERN_PROPERTIES    = "patternProperties"
	KEY_ADDITIONAL_PROPERTIES = "additionalProperties"
	KEY_PROPERTY_NAMES        = "propertyNames"
	KEY_DEFINITIONS           = "definitions"
	KEY_MULTIPLE_OF           = "multipleOf"
	KEY_MINIMUM               = "minimum"
	KEY_MAXIMUM               = "maximum"
	KEY_EXCLUSIVE_MINIMUM     = "exclusiveMinimum"
	KEY_EXCLUSIVE_MAXIMUM     = "exclusiveMaximum"
	KEY_MIN_LENGTH            = "minLength"
	KEY_MAX_LENGTH            = "maxLength"
	KEY_PATTERN               = "pattern"
	KEY_FORMAT                = "format"
	KEY_MIN_PROPERTIES        = "minProperties"
	KEY_MAX_PROPERTIES        = "maxProperties"
	KEY_DEPENDENCIES          = "dependencies"
	KEY_REQUIRED              = "required"
	KEY_MIN_ITEMS             = "minItems"
	KEY_MAX_ITEMS             = "maxItems"
	KEY_UNIQUE_ITEMS          = "uniqueItems"
	KEY_CONTAINS              = "contains"
	KEY_CONST                 = "const"
	KEY_ENUM                  = "enum"
	KEY_ONE_OF                = "oneOf"
	KEY_ANY_OF                = "anyOf"
	KEY_ALL_OF                = "allOf"
	KEY_NOT                   = "not"
	KEY_IF                    = "if"
	KEY_THEN                  = "then"
	KEY_ELSE                  = "else"
)

type SubSchema struct {
	Draft *Draft

	// basic SubSchema meta properties
	Id          *gojsonreference.JsonReference
	title       *string
	description *string

	Property string

	// Quick pass/fail for boolean schemas
	pass *bool

	// Types associated with the SubSchema
	Types jsonSchemaType

	// Reference url
	Ref *gojsonreference.JsonReference
	// Schema referenced
	RefSchema *SubSchema

	// hierarchy
	Parent                      *SubSchema
	ItemsChildren               []*SubSchema
	itemsChildrenIsSingleSchema bool
	PropertiesChildren          []*SubSchema

	// validation : number / integer
	multipleOf       *big.Rat
	maximum          *big.Rat
	exclusiveMaximum *big.Rat
	minimum          *big.Rat
	exclusiveMinimum *big.Rat

	// validation : string
	minLength *int
	maxLength *int
	pattern   *regexp.Regexp
	format    string

	// validation : object
	minProperties *int
	maxProperties *int
	required      []string

	dependencies         map[string]interface{}
	additionalProperties interface{}
	patternProperties    map[string]*SubSchema
	propertyNames        *SubSchema

	// validation : array
	minItems    *int
	maxItems    *int
	uniqueItems bool
	contains    *SubSchema

	additionalItems interface{}

	// validation : all
	_const *string //const is a golang keyword
	enum   []string

	// validation : SubSchema
	oneOf []*SubSchema
	anyOf []*SubSchema
	allOf []*SubSchema
	not   *SubSchema
	_if   *SubSchema // if/else are golang keywords
	_then *SubSchema
	_else *SubSchema
}
