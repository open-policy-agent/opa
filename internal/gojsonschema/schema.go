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
// description      Defines Schema, the main entry to every SubSchema.
//                  Contains the parsing logic and error checking.
//
// created          26-02-2013

package gojsonschema

import (
	"errors"
	"math/big"
	"reflect"
	"regexp"
	"text/template"

	"github.com/xeipuuv/gojsonreference"
)

var (
	// Locale is the default locale to use
	// Library users can overwrite with their own implementation
	Locale locale = DefaultLocale{}

	// ErrorTemplateFuncs allows you to define custom template funcs for use in localization.
	ErrorTemplateFuncs template.FuncMap
)

// NewSchema instances a schema using the given JSONLoader
func NewSchema(l JSONLoader) (*Schema, error) {
	return NewSchemaLoader().Compile(l)
}

// Schema holds a schema
type Schema struct {
	DocumentReference gojsonreference.JsonReference
	RootSchema        *SubSchema
	Pool              *schemaPool
	ReferencePool     *schemaReferencePool
}

func (d *Schema) parse(document interface{}, draft Draft) error {
	d.RootSchema = &SubSchema{Property: StringRootSchemaProperty, Draft: &draft}
	return d.parseSchema(document, d.RootSchema)
}

// SetRootSchemaName sets the root-schema name
func (d *Schema) SetRootSchemaName(name string) {
	d.RootSchema.Property = name
}

// Parses a SubSchema
//
// Pretty long function ( sorry :) )... but pretty straight forward, repetitive and boring
// Not much magic involved here, most of the job is to validate the key names and their values,
// then the values are copied into SubSchema struct
//
func (d *Schema) parseSchema(documentNode interface{}, currentSchema *SubSchema) error {

	if currentSchema.Draft == nil {
		if currentSchema.Parent == nil {
			return errors.New("Draft not set")
		}
		currentSchema.Draft = currentSchema.Parent.Draft
	}

	// As of draft 6 "true" is equivalent to an empty schema "{}" and false equals "{"not":{}}"
	if *currentSchema.Draft >= Draft6 && isKind(documentNode, reflect.Bool) {
		b := documentNode.(bool)
		currentSchema.pass = &b
		return nil
	}

	if !isKind(documentNode, reflect.Map) {
		return errors.New(formatErrorDescription(
			Locale.ParseError(),
			ErrorDetails{
				"expected": StringSchema,
			},
		))
	}

	m := documentNode.(map[string]interface{})

	if currentSchema.Parent == nil {
		currentSchema.Ref = &d.DocumentReference
		currentSchema.ID = &d.DocumentReference
	}

	if currentSchema.ID == nil && currentSchema.Parent != nil {
		currentSchema.ID = currentSchema.Parent.ID
	}

	// In draft 6 the id keyword was renamed to $id
	// Hybrid mode uses the old id by default
	var keyID string

	switch *currentSchema.Draft {
	case Draft4:
		keyID = KeyID
	case Hybrid:
		keyID = KeyIDNew
		if existsMapKey(m, KeyID) {
			keyID = KeyID
		}
	default:
		keyID = KeyIDNew
	}
	if existsMapKey(m, keyID) && !isKind(m[keyID], reflect.String) {
		return errors.New(formatErrorDescription(
			Locale.InvalidType(),
			ErrorDetails{
				"expected": TypeString,
				"given":    keyID,
			},
		))
	}
	if k, ok := m[keyID].(string); ok {
		jsonReference, err := gojsonreference.NewJsonReference(k)
		if err != nil {
			return err
		}
		if currentSchema == d.RootSchema {
			currentSchema.ID = &jsonReference
		} else {
			ref, err := currentSchema.Parent.ID.Inherits(jsonReference)
			if err != nil {
				return err
			}
			currentSchema.ID = ref
		}
	}

	// definitions
	if existsMapKey(m, KeyDefinitions) {
		if isKind(m[KeyDefinitions], reflect.Map, reflect.Bool) {
			for _, dv := range m[KeyDefinitions].(map[string]interface{}) {
				if isKind(dv, reflect.Map, reflect.Bool) {

					newSchema := &SubSchema{Property: KeyDefinitions, Parent: currentSchema}

					err := d.parseSchema(dv, newSchema)

					if err != nil {
						return err
					}
				} else {
					return errors.New(formatErrorDescription(
						Locale.InvalidType(),
						ErrorDetails{
							"expected": StringArrayOfSchemas,
							"given":    KeyDefinitions,
						},
					))
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": StringArrayOfSchemas,
					"given":    KeyDefinitions,
				},
			))
		}

	}

	// title
	if existsMapKey(m, KeyTitle) && !isKind(m[KeyTitle], reflect.String) {
		return errors.New(formatErrorDescription(
			Locale.InvalidType(),
			ErrorDetails{
				"expected": TypeString,
				"given":    KeyTitle,
			},
		))
	}
	if k, ok := m[KeyTitle].(string); ok {
		currentSchema.title = &k
	}

	// description
	if existsMapKey(m, KeyDescription) && !isKind(m[KeyDescription], reflect.String) {
		return errors.New(formatErrorDescription(
			Locale.InvalidType(),
			ErrorDetails{
				"expected": TypeString,
				"given":    KeyDescription,
			},
		))
	}
	if k, ok := m[KeyDescription].(string); ok {
		currentSchema.description = &k
	}

	// $ref
	if existsMapKey(m, KeyRef) && !isKind(m[KeyRef], reflect.String) {
		return errors.New(formatErrorDescription(
			Locale.InvalidType(),
			ErrorDetails{
				"expected": TypeString,
				"given":    KeyRef,
			},
		))
	}

	if k, ok := m[KeyRef].(string); ok {

		jsonReference, err := gojsonreference.NewJsonReference(k)
		if err != nil {
			return err
		}

		currentSchema.Ref = &jsonReference

		if sch, ok := d.ReferencePool.Get(currentSchema.Ref.String()); ok {
			currentSchema.RefSchema = sch
		} else {
			err := d.parseReference(documentNode, currentSchema)

			if err != nil {
				return err
			}

			return nil
		}
	}

	// type
	if existsMapKey(m, KeyType) {
		if isKind(m[KeyType], reflect.String) {
			if k, ok := m[KeyType].(string); ok {
				err := currentSchema.Types.Add(k)
				if err != nil {
					return err
				}
			}
		} else {
			if isKind(m[KeyType], reflect.Slice) {
				arrayOfTypes := m[KeyType].([]interface{})
				for _, typeInArray := range arrayOfTypes {
					if reflect.ValueOf(typeInArray).Kind() != reflect.String {
						return errors.New(formatErrorDescription(
							Locale.InvalidType(),
							ErrorDetails{
								"expected": TypeString + "/" + StringArrayOfStrings,
								"given":    KeyType,
							},
						))
					}
					if err := currentSchema.Types.Add(typeInArray.(string)); err != nil {
						return err
					}
				}

			} else {
				return errors.New(formatErrorDescription(
					Locale.InvalidType(),
					ErrorDetails{
						"expected": TypeString + "/" + StringArrayOfStrings,
						"given":    KeyType,
					},
				))
			}
		}
	}

	// properties
	if existsMapKey(m, KeyProperties) {
		err := d.parseProperties(m[KeyProperties], currentSchema)
		if err != nil {
			return err
		}
	}

	// additionalProperties
	if existsMapKey(m, KeyAdditionalProperties) {
		if isKind(m[KeyAdditionalProperties], reflect.Bool) {
			currentSchema.additionalProperties = m[KeyAdditionalProperties].(bool)
		} else if isKind(m[KeyAdditionalProperties], reflect.Map) {
			newSchema := &SubSchema{Property: KeyAdditionalProperties, Parent: currentSchema, Ref: currentSchema.Ref}
			currentSchema.additionalProperties = newSchema
			err := d.parseSchema(m[KeyAdditionalProperties], newSchema)
			if err != nil {
				return errors.New(err.Error())
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TypeBoolean + "/" + StringSchema,
					"given":    KeyAdditionalProperties,
				},
			))
		}
	}

	// patternProperties
	if existsMapKey(m, KeyPatternProperties) {
		if isKind(m[KeyPatternProperties], reflect.Map) {
			patternPropertiesMap := m[KeyPatternProperties].(map[string]interface{})
			if len(patternPropertiesMap) > 0 {
				currentSchema.patternProperties = make(map[string]*SubSchema)
				for k, v := range patternPropertiesMap {
					_, err := regexp.MatchString(k, "")
					if err != nil {
						return errors.New(formatErrorDescription(
							Locale.RegexPattern(),
							ErrorDetails{"pattern": k},
						))
					}
					newSchema := &SubSchema{Property: k, Parent: currentSchema, Ref: currentSchema.Ref}
					err = d.parseSchema(v, newSchema)
					if err != nil {
						return errors.New(err.Error())
					}
					currentSchema.patternProperties[k] = newSchema
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": StringSchema,
					"given":    KeyPatternProperties,
				},
			))
		}
	}

	// propertyNames
	if existsMapKey(m, KeyPropertyNames) && *currentSchema.Draft >= Draft6 {
		if isKind(m[KeyPropertyNames], reflect.Map, reflect.Bool) {
			newSchema := &SubSchema{Property: KeyPropertyNames, Parent: currentSchema, Ref: currentSchema.Ref}
			currentSchema.propertyNames = newSchema
			err := d.parseSchema(m[KeyPropertyNames], newSchema)
			if err != nil {
				return err
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": StringSchema,
					"given":    KeyPatternProperties,
				},
			))
		}
	}

	// dependencies
	if existsMapKey(m, KeyDependencies) {
		err := d.parseDependencies(m[KeyDependencies], currentSchema)
		if err != nil {
			return err
		}
	}

	// items
	if existsMapKey(m, KeyItems) {
		if isKind(m[KeyItems], reflect.Slice) {
			for _, itemElement := range m[KeyItems].([]interface{}) {
				if isKind(itemElement, reflect.Map, reflect.Bool) {
					newSchema := &SubSchema{Parent: currentSchema, Property: KeyItems}
					newSchema.Ref = currentSchema.Ref
					currentSchema.ItemsChildren = append(currentSchema.ItemsChildren, newSchema)
					err := d.parseSchema(itemElement, newSchema)
					if err != nil {
						return err
					}
				} else {
					return errors.New(formatErrorDescription(
						Locale.InvalidType(),
						ErrorDetails{
							"expected": StringSchema + "/" + StringArrayOfSchemas,
							"given":    KeyItems,
						},
					))
				}
				currentSchema.itemsChildrenIsSingleSchema = false
			}
		} else if isKind(m[KeyItems], reflect.Map, reflect.Bool) {
			newSchema := &SubSchema{Parent: currentSchema, Property: KeyItems}
			newSchema.Ref = currentSchema.Ref
			currentSchema.ItemsChildren = append(currentSchema.ItemsChildren, newSchema)
			err := d.parseSchema(m[KeyItems], newSchema)
			if err != nil {
				return err
			}
			currentSchema.itemsChildrenIsSingleSchema = true
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": StringSchema + "/" + StringArrayOfSchemas,
					"given":    KeyItems,
				},
			))
		}
	}

	// additionalItems
	if existsMapKey(m, KeyAdditionalItems) {
		if isKind(m[KeyAdditionalItems], reflect.Bool) {
			currentSchema.additionalItems = m[KeyAdditionalItems].(bool)
		} else if isKind(m[KeyAdditionalItems], reflect.Map) {
			newSchema := &SubSchema{Property: KeyAdditionalItems, Parent: currentSchema, Ref: currentSchema.Ref}
			currentSchema.additionalItems = newSchema
			err := d.parseSchema(m[KeyAdditionalItems], newSchema)
			if err != nil {
				return errors.New(err.Error())
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": TypeBoolean + "/" + StringSchema,
					"given":    KeyAdditionalItems,
				},
			))
		}
	}

	// validation : number / integer

	if existsMapKey(m, KeyMultipleOf) {
		multipleOfValue := mustBeNumber(m[KeyMultipleOf])
		if multipleOfValue == nil {
			return errors.New(formatErrorDescription(
				Locale.InvalidType(),
				ErrorDetails{
					"expected": StringNumber,
					"given":    KeyMultipleOf,
				},
			))
		}
		if multipleOfValue.Cmp(big.NewRat(0, 1)) <= 0 {
			return errors.New(formatErrorDescription(
				Locale.GreaterThanZero(),
				ErrorDetails{"number": KeyMultipleOf},
			))
		}
		currentSchema.multipleOf = multipleOfValue
	}

	if existsMapKey(m, KeyMinimum) {
		minimumValue := mustBeNumber(m[KeyMinimum])
		if minimumValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KeyMinimum, "y": StringNumber},
			))
		}
		currentSchema.minimum = minimumValue
	}

	if existsMapKey(m, KeyExclusiveMinimum) {
		switch *currentSchema.Draft {
		case Draft4:
			if !isKind(m[KeyExclusiveMinimum], reflect.Bool) {
				return errors.New(formatErrorDescription(
					Locale.InvalidType(),
					ErrorDetails{
						"expected": TypeBoolean,
						"given":    KeyExclusiveMinimum,
					},
				))
			}
			if currentSchema.minimum == nil {
				return errors.New(formatErrorDescription(
					Locale.CannotBeUsedWithout(),
					ErrorDetails{"x": KeyExclusiveMinimum, "y": KeyMinimum},
				))
			}
			if m[KeyExclusiveMinimum].(bool) {
				currentSchema.exclusiveMinimum = currentSchema.minimum
				currentSchema.minimum = nil
			}
		case Hybrid:
			if isKind(m[KeyExclusiveMinimum], reflect.Bool) {
				if currentSchema.minimum == nil {
					return errors.New(formatErrorDescription(
						Locale.CannotBeUsedWithout(),
						ErrorDetails{"x": KeyExclusiveMinimum, "y": KeyMinimum},
					))
				}
				if m[KeyExclusiveMinimum].(bool) {
					currentSchema.exclusiveMinimum = currentSchema.minimum
					currentSchema.minimum = nil
				}
			} else if isJSONNumber(m[KeyExclusiveMinimum]) {
				currentSchema.exclusiveMinimum = mustBeNumber(m[KeyExclusiveMinimum])
			} else {
				return errors.New(formatErrorDescription(
					Locale.InvalidType(),
					ErrorDetails{
						"expected": TypeBoolean + "/" + TypeNumber,
						"given":    KeyExclusiveMinimum,
					},
				))
			}
		default:
			if isJSONNumber(m[KeyExclusiveMinimum]) {
				currentSchema.exclusiveMinimum = mustBeNumber(m[KeyExclusiveMinimum])
			} else {
				return errors.New(formatErrorDescription(
					Locale.InvalidType(),
					ErrorDetails{
						"expected": TypeNumber,
						"given":    KeyExclusiveMinimum,
					},
				))
			}
		}
	}

	if existsMapKey(m, KeyMaximum) {
		maximumValue := mustBeNumber(m[KeyMaximum])
		if maximumValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KeyMaximum, "y": StringNumber},
			))
		}
		currentSchema.maximum = maximumValue
	}

	if existsMapKey(m, KeyExclusiveMaximum) {
		switch *currentSchema.Draft {
		case Draft4:
			if !isKind(m[KeyExclusiveMaximum], reflect.Bool) {
				return errors.New(formatErrorDescription(
					Locale.InvalidType(),
					ErrorDetails{
						"expected": TypeBoolean,
						"given":    KeyExclusiveMaximum,
					},
				))
			}
			if currentSchema.maximum == nil {
				return errors.New(formatErrorDescription(
					Locale.CannotBeUsedWithout(),
					ErrorDetails{"x": KeyExclusiveMaximum, "y": KeyMaximum},
				))
			}
			if m[KeyExclusiveMaximum].(bool) {
				currentSchema.exclusiveMaximum = currentSchema.maximum
				currentSchema.maximum = nil
			}
		case Hybrid:
			if isKind(m[KeyExclusiveMaximum], reflect.Bool) {
				if currentSchema.maximum == nil {
					return errors.New(formatErrorDescription(
						Locale.CannotBeUsedWithout(),
						ErrorDetails{"x": KeyExclusiveMaximum, "y": KeyMaximum},
					))
				}
				if m[KeyExclusiveMaximum].(bool) {
					currentSchema.exclusiveMaximum = currentSchema.maximum
					currentSchema.maximum = nil
				}
			} else if isJSONNumber(m[KeyExclusiveMaximum]) {
				currentSchema.exclusiveMaximum = mustBeNumber(m[KeyExclusiveMaximum])
			} else {
				return errors.New(formatErrorDescription(
					Locale.InvalidType(),
					ErrorDetails{
						"expected": TypeBoolean + "/" + TypeNumber,
						"given":    KeyExclusiveMaximum,
					},
				))
			}
		default:
			if isJSONNumber(m[KeyExclusiveMaximum]) {
				currentSchema.exclusiveMaximum = mustBeNumber(m[KeyExclusiveMaximum])
			} else {
				return errors.New(formatErrorDescription(
					Locale.InvalidType(),
					ErrorDetails{
						"expected": TypeNumber,
						"given":    KeyExclusiveMaximum,
					},
				))
			}
		}
	}

	// validation : string

	if existsMapKey(m, KeyMinLength) {
		minLengthIntegerValue := mustBeInteger(m[KeyMinLength])
		if minLengthIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyMinLength, "y": TypeInteger},
			))
		}
		if *minLengthIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KeyMinLength},
			))
		}
		currentSchema.minLength = minLengthIntegerValue
	}

	if existsMapKey(m, KeyMaxLength) {
		maxLengthIntegerValue := mustBeInteger(m[KeyMaxLength])
		if maxLengthIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyMaxLength, "y": TypeInteger},
			))
		}
		if *maxLengthIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KeyMaxLength},
			))
		}
		currentSchema.maxLength = maxLengthIntegerValue
	}

	if currentSchema.minLength != nil && currentSchema.maxLength != nil {
		if *currentSchema.minLength > *currentSchema.maxLength {
			return errors.New(formatErrorDescription(
				Locale.CannotBeGT(),
				ErrorDetails{"x": KeyMinLength, "y": KeyMaxLength},
			))
		}
	}

	if existsMapKey(m, KeyPattern) {
		if isKind(m[KeyPattern], reflect.String) {
			regexpObject, err := regexp.Compile(m[KeyPattern].(string))
			if err != nil {
				return errors.New(formatErrorDescription(
					Locale.MustBeValidRegex(),
					ErrorDetails{"key": KeyPattern},
				))
			}
			currentSchema.pattern = regexpObject
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KeyPattern, "y": TypeString},
			))
		}
	}

	if existsMapKey(m, KeyFormat) {
		formatString, ok := m[KeyFormat].(string)
		if !ok {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfType(),
				ErrorDetails{"key": KeyFormat, "type": TypeString},
			))
		}
		currentSchema.format = formatString
	}

	// validation : object

	if existsMapKey(m, KeyMinProperties) {
		minPropertiesIntegerValue := mustBeInteger(m[KeyMinProperties])
		if minPropertiesIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyMinProperties, "y": TypeInteger},
			))
		}
		if *minPropertiesIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KeyMinProperties},
			))
		}
		currentSchema.minProperties = minPropertiesIntegerValue
	}

	if existsMapKey(m, KeyMaxProperties) {
		maxPropertiesIntegerValue := mustBeInteger(m[KeyMaxProperties])
		if maxPropertiesIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyMaxProperties, "y": TypeInteger},
			))
		}
		if *maxPropertiesIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KeyMaxProperties},
			))
		}
		currentSchema.maxProperties = maxPropertiesIntegerValue
	}

	if currentSchema.minProperties != nil && currentSchema.maxProperties != nil {
		if *currentSchema.minProperties > *currentSchema.maxProperties {
			return errors.New(formatErrorDescription(
				Locale.KeyCannotBeGreaterThan(),
				ErrorDetails{"key": KeyMinProperties, "y": KeyMaxProperties},
			))
		}
	}

	if existsMapKey(m, KeyRequired) {
		if isKind(m[KeyRequired], reflect.Slice) {
			requiredValues := m[KeyRequired].([]interface{})
			for _, requiredValue := range requiredValues {
				if isKind(requiredValue, reflect.String) {
					if isStringInSlice(currentSchema.required, requiredValue.(string)) {
						return errors.New(formatErrorDescription(
							Locale.KeyItemsMustBeUnique(),
							ErrorDetails{"key": KeyRequired},
						))
					}
					currentSchema.required = append(currentSchema.required, requiredValue.(string))
				} else {
					return errors.New(formatErrorDescription(
						Locale.KeyItemsMustBeOfType(),
						ErrorDetails{"key": KeyRequired, "type": TypeString},
					))
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyRequired, "y": TypeArray},
			))
		}
	}

	// validation : array

	if existsMapKey(m, KeyMinItems) {
		minItemsIntegerValue := mustBeInteger(m[KeyMinItems])
		if minItemsIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyMinItems, "y": TypeInteger},
			))
		}
		if *minItemsIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KeyMinItems},
			))
		}
		currentSchema.minItems = minItemsIntegerValue
	}

	if existsMapKey(m, KeyMaxItems) {
		maxItemsIntegerValue := mustBeInteger(m[KeyMaxItems])
		if maxItemsIntegerValue == nil {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyMaxItems, "y": TypeInteger},
			))
		}
		if *maxItemsIntegerValue < 0 {
			return errors.New(formatErrorDescription(
				Locale.MustBeGTEZero(),
				ErrorDetails{"key": KeyMaxItems},
			))
		}
		currentSchema.maxItems = maxItemsIntegerValue
	}

	if existsMapKey(m, KeyUniqueItems) {
		if isKind(m[KeyUniqueItems], reflect.Bool) {
			currentSchema.uniqueItems = m[KeyUniqueItems].(bool)
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfA(),
				ErrorDetails{"x": KeyUniqueItems, "y": TypeBoolean},
			))
		}
	}

	if existsMapKey(m, KeyContains) && *currentSchema.Draft >= Draft6 {
		newSchema := &SubSchema{Property: KeyContains, Parent: currentSchema, Ref: currentSchema.Ref}
		currentSchema.contains = newSchema
		err := d.parseSchema(m[KeyContains], newSchema)
		if err != nil {
			return err
		}
	}

	// validation : all

	if existsMapKey(m, KeyConst) && *currentSchema.Draft >= Draft6 {
		is, err := marshalWithoutNumber(m[KeyConst])
		if err != nil {
			return err
		}
		currentSchema._const = is
	}

	if existsMapKey(m, KeyEnum) {
		if isKind(m[KeyEnum], reflect.Slice) {
			for _, v := range m[KeyEnum].([]interface{}) {
				is, err := marshalWithoutNumber(v)
				if err != nil {
					return err
				}
				if isStringInSlice(currentSchema.enum, *is) {
					return errors.New(formatErrorDescription(
						Locale.KeyItemsMustBeUnique(),
						ErrorDetails{"key": KeyEnum},
					))
				}
				currentSchema.enum = append(currentSchema.enum, *is)
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyEnum, "y": TypeArray},
			))
		}
	}

	// validation : SubSchema

	if existsMapKey(m, KeyOneOf) {
		if isKind(m[KeyOneOf], reflect.Slice) {
			for _, v := range m[KeyOneOf].([]interface{}) {
				newSchema := &SubSchema{Property: KeyOneOf, Parent: currentSchema, Ref: currentSchema.Ref}
				currentSchema.oneOf = append(currentSchema.oneOf, newSchema)
				err := d.parseSchema(v, newSchema)
				if err != nil {
					return err
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyOneOf, "y": TypeArray},
			))
		}
	}

	if existsMapKey(m, KeyAnyOf) {
		if isKind(m[KeyAnyOf], reflect.Slice) {
			for _, v := range m[KeyAnyOf].([]interface{}) {
				newSchema := &SubSchema{Property: KeyAnyOf, Parent: currentSchema, Ref: currentSchema.Ref}
				currentSchema.anyOf = append(currentSchema.anyOf, newSchema)
				err := d.parseSchema(v, newSchema)
				if err != nil {
					return err
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyAnyOf, "y": TypeArray},
			))
		}
	}

	if existsMapKey(m, KeyAllOf) {
		if isKind(m[KeyAllOf], reflect.Slice) {
			for _, v := range m[KeyAllOf].([]interface{}) {
				newSchema := &SubSchema{Property: KeyAllOf, Parent: currentSchema, Ref: currentSchema.Ref}
				currentSchema.allOf = append(currentSchema.allOf, newSchema)
				err := d.parseSchema(v, newSchema)
				if err != nil {
					return err
				}
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyAnyOf, "y": TypeArray},
			))
		}
	}

	if existsMapKey(m, KeyNot) {
		if isKind(m[KeyNot], reflect.Map, reflect.Bool) {
			newSchema := &SubSchema{Property: KeyNot, Parent: currentSchema, Ref: currentSchema.Ref}
			currentSchema.not = newSchema
			err := d.parseSchema(m[KeyNot], newSchema)
			if err != nil {
				return err
			}
		} else {
			return errors.New(formatErrorDescription(
				Locale.MustBeOfAn(),
				ErrorDetails{"x": KeyNot, "y": TypeObject},
			))
		}
	}

	if *currentSchema.Draft >= Draft7 {
		if existsMapKey(m, KeyIf) {
			if isKind(m[KeyIf], reflect.Map, reflect.Bool) {
				newSchema := &SubSchema{Property: KeyIf, Parent: currentSchema, Ref: currentSchema.Ref}
				currentSchema._if = newSchema
				err := d.parseSchema(m[KeyIf], newSchema)
				if err != nil {
					return err
				}
			} else {
				return errors.New(formatErrorDescription(
					Locale.MustBeOfAn(),
					ErrorDetails{"x": KeyIf, "y": TypeObject},
				))
			}
		}

		if existsMapKey(m, KeyThen) {
			if isKind(m[KeyThen], reflect.Map, reflect.Bool) {
				newSchema := &SubSchema{Property: KeyThen, Parent: currentSchema, Ref: currentSchema.Ref}
				currentSchema._then = newSchema
				err := d.parseSchema(m[KeyThen], newSchema)
				if err != nil {
					return err
				}
			} else {
				return errors.New(formatErrorDescription(
					Locale.MustBeOfAn(),
					ErrorDetails{"x": KeyThen, "y": TypeObject},
				))
			}
		}

		if existsMapKey(m, KeyElse) {
			if isKind(m[KeyElse], reflect.Map, reflect.Bool) {
				newSchema := &SubSchema{Property: KeyElse, Parent: currentSchema, Ref: currentSchema.Ref}
				currentSchema._else = newSchema
				err := d.parseSchema(m[KeyElse], newSchema)
				if err != nil {
					return err
				}
			} else {
				return errors.New(formatErrorDescription(
					Locale.MustBeOfAn(),
					ErrorDetails{"x": KeyElse, "y": TypeObject},
				))
			}
		}
	}

	return nil
}

func (d *Schema) parseReference(documentNode interface{}, currentSchema *SubSchema) error {
	var (
		refdDocumentNode interface{}
		dsp              *schemaPoolDocument
		err              error
	)

	newSchema := &SubSchema{Property: KeyRef, Parent: currentSchema, Ref: currentSchema.Ref}

	d.ReferencePool.Add(currentSchema.Ref.String(), newSchema)

	dsp, err = d.Pool.GetDocument(*currentSchema.Ref)
	if err != nil {
		return err
	}
	newSchema.ID = currentSchema.Ref

	refdDocumentNode = dsp.Document
	newSchema.Draft = dsp.Draft

	if err != nil {
		return err
	}

	if !isKind(refdDocumentNode, reflect.Map, reflect.Bool) {
		return errors.New(formatErrorDescription(
			Locale.MustBeOfType(),
			ErrorDetails{"key": StringSchema, "type": TypeObject},
		))
	}

	err = d.parseSchema(refdDocumentNode, newSchema)
	if err != nil {
		return err
	}

	currentSchema.RefSchema = newSchema

	return nil

}

func (d *Schema) parseProperties(documentNode interface{}, currentSchema *SubSchema) error {

	if !isKind(documentNode, reflect.Map) {
		return errors.New(formatErrorDescription(
			Locale.MustBeOfType(),
			ErrorDetails{"key": StringProperties, "type": TypeObject},
		))
	}

	m := documentNode.(map[string]interface{})
	for k := range m {
		schemaProperty := k
		newSchema := &SubSchema{Property: schemaProperty, Parent: currentSchema, Ref: currentSchema.Ref}
		currentSchema.PropertiesChildren = append(currentSchema.PropertiesChildren, newSchema)
		err := d.parseSchema(m[k], newSchema)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Schema) parseDependencies(documentNode interface{}, currentSchema *SubSchema) error {

	if !isKind(documentNode, reflect.Map) {
		return errors.New(formatErrorDescription(
			Locale.MustBeOfType(),
			ErrorDetails{"key": KeyDependencies, "type": TypeObject},
		))
	}

	m := documentNode.(map[string]interface{})
	currentSchema.dependencies = make(map[string]interface{})

	for k := range m {
		switch reflect.ValueOf(m[k]).Kind() {

		case reflect.Slice:
			values := m[k].([]interface{})
			var valuesToRegister []string

			for _, value := range values {
				if !isKind(value, reflect.String) {
					return errors.New(formatErrorDescription(
						Locale.MustBeOfType(),
						ErrorDetails{
							"key":  StringDependency,
							"type": StringSchemaOrArrayOfStrings,
						},
					))
				}
				valuesToRegister = append(valuesToRegister, value.(string))
				currentSchema.dependencies[k] = valuesToRegister
			}

		case reflect.Map, reflect.Bool:
			depSchema := &SubSchema{Property: k, Parent: currentSchema, Ref: currentSchema.Ref}
			err := d.parseSchema(m[k], depSchema)
			if err != nil {
				return err
			}
			currentSchema.dependencies[k] = depSchema

		default:
			return errors.New(formatErrorDescription(
				Locale.MustBeOfType(),
				ErrorDetails{
					"key":  StringDependency,
					"type": StringSchemaOrArrayOfStrings,
				},
			))
		}

	}

	return nil
}
