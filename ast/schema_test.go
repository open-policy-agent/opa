// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

func testParseSchema(t *testing.T, schema string, expectedType types.Type, expectedError error) {
	t.Helper()

	var sch interface{}
	err := util.Unmarshal([]byte(schema), &sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	newtype, err := loadSchema(sch, nil)
	if err != nil && errors.Is(err, expectedError) {
		t.Fatalf("unexpected error: %v", err)
	}
	if newtype == nil && expectedType != nil {
		t.Fatalf("parseSchema returned nil type")
	}
	if newtype != nil && expectedType == nil {
		t.Fatalf("expected nil but parseSchema returned a not nil type")
	}
	if types.Compare(newtype, expectedType) != 0 {
		t.Fatalf("parseSchema returned an incorrect type: %s, expected: %s", newtype.String(), expectedType.String())
	}
}

func TestParseSchemaObject(t *testing.T) {
	innerObjectStaticProps := []*types.StaticProperty{}
	innerObjectStaticProps = append(innerObjectStaticProps, &types.StaticProperty{Key: "a", Value: types.N})
	innerObjectStaticProps = append(innerObjectStaticProps, &types.StaticProperty{Key: "b", Value: types.NewArray(nil, types.N)})
	innerObjectStaticProps = append(innerObjectStaticProps, &types.StaticProperty{Key: "c", Value: types.A})
	innerObjectType := types.NewObject(innerObjectStaticProps, nil)

	staticProps := []*types.StaticProperty{}
	staticProps = append(staticProps, &types.StaticProperty{Key: "b", Value: types.NewArray(nil, innerObjectType)})
	staticProps = append(staticProps, &types.StaticProperty{Key: "foo", Value: types.S})

	expectedType := types.NewObject(staticProps, nil)
	testParseSchema(t, objectSchema, expectedType, nil)
}

func TestSetTypesWithSchemaRef(t *testing.T) {
	var sch interface{}

	ts := kubeSchemaServer(t)
	t.Cleanup(ts.Close)
	refSchemaReplaced := strings.ReplaceAll(refSchema, "https://kubernetesjsonschema.dev/v1.14.0/", ts.URL+"/")
	err := util.Unmarshal([]byte(refSchemaReplaced), &sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	t.Run("remote refs disabled", func(t *testing.T) {
		_, err := loadSchema(sch, []string{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		expErr := fmt.Sprintf("unable to compile the schema: remote reference loading disabled: %s/_definitions.json", ts.URL)
		if exp, act := expErr, err.Error(); act != exp {
			t.Errorf("expected message %q, got %q", exp, act)
		}
	})

	t.Run("all remote refs enabled", func(t *testing.T) {
		newtype, err := loadSchema(sch, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if newtype == nil {
			t.Fatalf("parseSchema returned nil type")
		}
		if newtype.String() != "object<apiVersion: string, kind: string, metadata: object<annotations: object[any: any], clusterName: string, creationTimestamp: string, deletionGracePeriodSeconds: number, deletionTimestamp: string, finalizers: array[string], generateName: string, generation: number, initializers: object<pending: array[object<name: string>], result: object<apiVersion: string, code: number, details: object<causes: array[object<field: string, message: string, reason: string>], group: string, kind: string, name: string, retryAfterSeconds: number, uid: string>, kind: string, message: string, metadata: object<continue: string, resourceVersion: string, selfLink: string>, reason: string, status: string>>, labels: object[any: any], managedFields: array[object<apiVersion: string, fields: object[any: any], manager: string, operation: string, time: string>], name: string, namespace: string, ownerReferences: array[object<apiVersion: string, blockOwnerDeletion: boolean, controller: boolean, kind: string, name: string, uid: string>], resourceVersion: string, selfLink: string, uid: string>>" {
			t.Fatalf("parseSchema returned an incorrect type: %s", newtype.String())
		}
	})

	t.Run("desired remote ref selectively enabled", func(t *testing.T) {
		newtype, err := loadSchema(sch, []string{"127.0.0.1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if newtype == nil {
			t.Fatalf("parseSchema returned nil type")
		}
		if newtype.String() != "object<apiVersion: string, kind: string, metadata: object<annotations: object[any: any], clusterName: string, creationTimestamp: string, deletionGracePeriodSeconds: number, deletionTimestamp: string, finalizers: array[string], generateName: string, generation: number, initializers: object<pending: array[object<name: string>], result: object<apiVersion: string, code: number, details: object<causes: array[object<field: string, message: string, reason: string>], group: string, kind: string, name: string, retryAfterSeconds: number, uid: string>, kind: string, message: string, metadata: object<continue: string, resourceVersion: string, selfLink: string>, reason: string, status: string>>, labels: object[any: any], managedFields: array[object<apiVersion: string, fields: object[any: any], manager: string, operation: string, time: string>], name: string, namespace: string, ownerReferences: array[object<apiVersion: string, blockOwnerDeletion: boolean, controller: boolean, kind: string, name: string, uid: string>], resourceVersion: string, selfLink: string, uid: string>>" {
			t.Fatalf("parseSchema returned an incorrect type: %s", newtype.String())
		}
	})

	t.Run("different remote ref selectively enabled", func(t *testing.T) {
		_, err := loadSchema(sch, []string{"foo"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		expErr := fmt.Sprintf("unable to compile the schema: remote reference loading disabled: %s/_definitions.json", ts.URL)
		if exp, act := expErr, err.Error(); act != exp {
			t.Errorf("expected message %q, got %q", exp, act)
		}
	})
}

func TestSetTypesWithPodSchema(t *testing.T) {
	var sch interface{}

	ts := kubeSchemaServer(t)
	t.Cleanup(ts.Close)

	podSchemaReplaced := strings.ReplaceAll(podSchema, "https://kubernetesjsonschema.dev/v1.14.0/", ts.URL+"/")
	err := util.Unmarshal([]byte(podSchemaReplaced), &sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	newtype, err := loadSchema(sch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if newtype == nil {
		t.Fatalf("parseSchema returned nil type")
	}
	if newtype.String() == "object<apiVersion: string, kind: string, metadata: any, spec: any, status: any>" {
		t.Fatalf("parseSchema returned an incorrect type: %s", newtype.String())
	}

}

func TestAllOfSchemas(t *testing.T) {
	//Test 1: object schema
	objectSchemaStaticProps := []*types.StaticProperty{}
	objectSchemaStaticProps = append(objectSchemaStaticProps, &types.StaticProperty{Key: "AddressLine1", Value: types.S})
	objectSchemaStaticProps = append(objectSchemaStaticProps, &types.StaticProperty{Key: "AddressLine2", Value: types.S})
	objectSchemaStaticProps = append(objectSchemaStaticProps, &types.StaticProperty{Key: "City", Value: types.S})
	objectSchemaStaticProps = append(objectSchemaStaticProps, &types.StaticProperty{Key: "State", Value: types.S})
	objectSchemaStaticProps = append(objectSchemaStaticProps, &types.StaticProperty{Key: "ZipCode", Value: types.S})
	objectSchemaStaticProps = append(objectSchemaStaticProps, &types.StaticProperty{Key: "County", Value: types.S})
	objectSchemaStaticProps = append(objectSchemaStaticProps, &types.StaticProperty{Key: "PostCode", Value: types.S})
	objectSchemaExpectedType := types.NewObject(objectSchemaStaticProps, nil)

	//Test 2: array schema
	arrayExpectedType := types.NewArray(nil, types.N)

	//Test 3: parent variation
	parentVariationStaticProps := []*types.StaticProperty{}
	parentVariationStaticProps = append(parentVariationStaticProps, &types.StaticProperty{Key: "State", Value: types.S})
	parentVariationStaticProps = append(parentVariationStaticProps, &types.StaticProperty{Key: "ZipCode", Value: types.S})
	parentVariationStaticProps = append(parentVariationStaticProps, &types.StaticProperty{Key: "County", Value: types.S})
	parentVariationStaticProps = append(parentVariationStaticProps, &types.StaticProperty{Key: "PostCode", Value: types.S})
	parentVariationExpectedType := types.NewObject(parentVariationStaticProps, nil)

	//Test 4: empty schema with allOf
	emptyExpectedType := types.A

	//Tests 5 & 6: schema with array of arrays,  object and array as siblings
	expectedError := fmt.Errorf("unable to merge these schemas")

	//Test 7: array of objects
	arrayOfObjectsStaticProps := []*types.StaticProperty{}
	arrayOfObjectsStaticProps = append(arrayOfObjectsStaticProps, &types.StaticProperty{Key: "State", Value: types.S})
	arrayOfObjectsStaticProps = append(arrayOfObjectsStaticProps, &types.StaticProperty{Key: "ZipCode", Value: types.S})
	arrayOfObjectsStaticProps = append(arrayOfObjectsStaticProps, &types.StaticProperty{Key: "County", Value: types.S})
	arrayOfObjectsStaticProps = append(arrayOfObjectsStaticProps, &types.StaticProperty{Key: "PostCode", Value: types.S})
	arrayOfObjectsStaticProps = append(arrayOfObjectsStaticProps, &types.StaticProperty{Key: "Street", Value: types.S})
	arrayOfObjectsStaticProps = append(arrayOfObjectsStaticProps, &types.StaticProperty{Key: "House", Value: types.S})
	innerType := types.NewObject(arrayOfObjectsStaticProps, nil)
	arrayOfObjectsExpectedType := types.NewArray(nil, innerType)

	//Tests 8 & 9: allOf schema with type not specified
	objectMissingStaticProps := []*types.StaticProperty{}
	objectMissingStaticProps = append(objectMissingStaticProps, &types.StaticProperty{Key: "AddressLine", Value: types.S})
	objectMissingStaticProps = append(objectMissingStaticProps, &types.StaticProperty{Key: "State", Value: types.S})
	objectMissingStaticProps = append(objectMissingStaticProps, &types.StaticProperty{Key: "ZipCode", Value: types.S})
	objectMissingStaticProps = append(objectMissingStaticProps, &types.StaticProperty{Key: "County", Value: types.S})
	objectMissingStaticProps = append(objectMissingStaticProps, &types.StaticProperty{Key: "PostCode", Value: types.N})
	objectMissingExpectedType := types.NewObject(objectMissingStaticProps, nil)
	arrayMissingExpectedType := types.NewArray([]types.Type{types.N, types.N}, nil)

	//Tests 10 & 11: allOf schema with array that contains different types (with and without error)
	arrayDifTypesExpectedType := types.NewArray([]types.Type{types.S, types.N}, nil)

	//Test 12: array inside of object
	arrayInObjectstaticProps := []*types.StaticProperty{}
	arrayInObjectstaticProps = append(arrayInObjectstaticProps, &types.StaticProperty{Key: "age", Value: types.N})
	arrayInObjectstaticProps = append(arrayInObjectstaticProps, &types.StaticProperty{Key: "name", Value: types.S})
	arrayInObjectstaticProps = append(arrayInObjectstaticProps, &types.StaticProperty{Key: "personality", Value: types.S})
	arrayInObjectstaticProps = append(arrayInObjectstaticProps, &types.StaticProperty{Key: "nickname", Value: types.S})
	innerObjectsType := types.NewObject(arrayInObjectstaticProps, nil)
	arrayInObjectInnerType := types.NewArray(nil, innerObjectsType)
	arrayInObjectExpectedType := types.NewObject([]*types.StaticProperty{
		types.NewStaticProperty("familyMembers", arrayInObjectInnerType)}, nil)

	//Test 13: allOf inside core schema
	coreStaticProps := []*types.StaticProperty{}
	coreStaticProps = append(coreStaticProps, &types.StaticProperty{Key: "accessMe", Value: types.S})
	coreStaticProps = append(coreStaticProps, &types.StaticProperty{Key: "accessYou", Value: types.S})
	insideType := types.NewObject(coreStaticProps, nil)
	outerType := []*types.StaticProperty{}
	outerType = append(outerType, &types.StaticProperty{Key: "RandomInfo", Value: insideType})
	outerType = append(outerType, &types.StaticProperty{Key: "AddressLine", Value: types.S})
	coreSchemaExpectedType := types.NewObject(outerType, nil)

	//Tests 14-17: other types besides array and object
	expectedStringType := types.NewString()
	expectedIntegerType := types.NewNumber()
	expectedBooleanType := types.NewBoolean()

	//Test 18: array with uneven numbers of items children to merge
	expectedUnevenArrayType := types.NewArray([]types.Type{types.N, types.N, types.S}, nil)

	tests := []struct {
		note          string
		schema        string
		expectedType  types.Type
		expectedError error
	}{
		{
			note:          "allOf with mergeable Object types in schema",
			schema:        allOfObjectSchema,
			expectedType:  objectSchemaExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array types in schema",
			schema:        allOfArraySchema,
			expectedType:  arrayExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf without a parent schema",
			schema:        allOfSchemaParentVariation,
			expectedType:  parentVariationExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf with empty schema",
			schema:        emptySchema,
			expectedType:  emptyExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf schema with unmergeable Array of Arrays",
			schema:        allOfArrayOfArrays,
			expectedType:  nil,
			expectedError: expectedError,
		},
		{
			note:          "allOf with mergeable Array of Object types in schema",
			schema:        allOfArrayOfObjects,
			expectedType:  arrayOfObjectsExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf schema with Array and Object types as siblings",
			schema:        allOfObjectAndArray,
			expectedType:  nil,
			expectedError: expectedError,
		},
		{
			note:          "allOf with mergeable Object types in schema with type declaration missing",
			schema:        allOfObjectMissing,
			expectedType:  objectMissingExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array types in schema with type declaration missing",
			schema:        allOfArrayMissing,
			expectedType:  arrayMissingExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf schema with an Array that contains different mergeable types",
			schema:        allOfArrayDifTypes,
			expectedType:  arrayDifTypesExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf schema with Array type that contains different unmergeable types",
			schema:        allOfArrayDifTypesWithError,
			expectedType:  nil,
			expectedError: expectedError,
		},
		{
			note:          "allOf with mergeable Object containing Array types in schema",
			schema:        allOfArrayInsideObject,
			expectedType:  arrayInObjectExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable types inside of core schema",
			schema:        allOfInsideCoreSchema,
			expectedType:  coreSchemaExpectedType,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable String types in schema",
			schema:        allOfStringSchema,
			expectedType:  expectedStringType,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Integer types in schema",
			schema:        allOfIntegerSchema,
			expectedType:  expectedIntegerType,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Boolean types in schema",
			schema:        allOfBooleanSchema,
			expectedType:  expectedBooleanType,
			expectedError: nil,
		},
		{
			note:          "allOf schema with different unmergeable types",
			schema:        allOfTypeErrorSchema,
			expectedType:  nil,
			expectedError: expectedError,
		},
		{
			note:          "allOf schema with unmergeable types String and Boolean",
			schema:        allOfStringSchemaWithError,
			expectedType:  nil,
			expectedError: expectedError,
		},
		{
			note:          "allOf unmergeable schema with different parent and items types",
			schema:        allOfSchemaWithParentError,
			expectedType:  nil,
			expectedError: expectedError,
		},
		{
			note:          "allOf schema of Array type with uneven numbers of items to merge",
			schema:        allOfSchemaWithUnevenArray,
			expectedType:  expectedUnevenArrayType,
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			testParseSchema(t, tc.schema, tc.expectedType, tc.expectedError)
		})
	}
}

func TestParseSchemaUntypedField(t *testing.T) {
	//Expected type is: object<foo: any>
	staticProps := []*types.StaticProperty{}
	staticProps = append(staticProps, &types.StaticProperty{Key: "foo", Value: types.A})
	expectedType := types.NewObject(staticProps, nil)
	testParseSchema(t, untypedFieldObjectSchema, expectedType, nil)
}

func TestParseSchemaNoChildren(t *testing.T) {
	//Expected type is: object[any: any]
	expectedType := types.NewObject(nil, &types.DynamicProperty{Key: types.A, Value: types.A})
	testParseSchema(t, noChildrenObjectSchema, expectedType, nil)
}

func TestParseSchemaArrayNoItems(t *testing.T) {
	//Expected type is: object<b: array[any]>
	staticProps := []*types.StaticProperty{}
	staticProps = append(staticProps, &types.StaticProperty{Key: "b", Value: types.NewArray(nil, types.A)})
	expectedType := types.NewObject(staticProps, nil)
	testParseSchema(t, arrayNoItemsSchema, expectedType, nil)
}

func TestParseSchemaBooleanField(t *testing.T) {
	//Expected type is: object<a: boolean>
	staticProps := []*types.StaticProperty{}
	staticProps = append(staticProps, &types.StaticProperty{Key: "a", Value: types.B})
	expectedType := types.NewObject(staticProps, nil)
	testParseSchema(t, booleanSchema, expectedType, nil)
}

func TestParseSchemaBasics(t *testing.T) {
	tests := []struct {
		note   string
		schema string
		exp    types.Type
	}{
		{
			note:   "number",
			schema: `{"type": "number"}`,
			exp:    types.N,
		},
		{
			note: "array of objects",
			schema: `{
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "string"},
						"value": {"type": "number"}
					}
				}
			}`,
			exp: types.NewArray(nil, types.NewObject([]*types.StaticProperty{
				types.NewStaticProperty("id", types.S),
				types.NewStaticProperty("value", types.N),
			}, nil)),
		},
		{
			note: "static array items",
			schema: `{
				"type": "array",
				"items": [
					{"type": "string"},
					{"type": "number"}
				]
			}`,
			exp: types.NewArray([]types.Type{
				types.S,
				types.N,
			}, nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			testParseSchema(t, tc.schema, tc.exp, nil)
		})
	}
}

func TestCompileSchemaEmptySchema(t *testing.T) {
	schema := ""
	var sch interface{}
	err := util.Unmarshal([]byte(schema), &sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	jsonSchema, _ := compileSchema(sch, []string{})
	if jsonSchema != nil {
		t.Fatalf("Incorrect return from parseSchema with an empty schema")
	}
}

func TestParseSchemaWithSchemaBadSchema(t *testing.T) {
	var sch interface{}
	err := util.Unmarshal([]byte(objectSchema), &sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	jsonSchema, err := compileSchema(sch, []string{})
	if err != nil {
		t.Fatalf("Unable to compile schema: %v", err)
	}
	newtype, err := newSchemaParser().parseSchema(jsonSchema) // Did not pass the subschema
	if err == nil {
		t.Fatalf("Expected parseSchema() = error, got nil")
	}
	if newtype != nil {
		t.Fatalf("Incorrect return from parseSchema with a bad schema")
	}
}

func TestAnyOfSchema(t *testing.T) {

	// Test 1 & 3: anyOf extends the core schema
	extendCoreStaticPropsExp1 := []*types.StaticProperty{types.NewStaticProperty("AddressLine", types.S)}
	extendCoreStaticPropsExp2 := []*types.StaticProperty{
		types.NewStaticProperty("State", types.S),
		types.NewStaticProperty("ZipCode", types.S)}
	extendCoreStaticPropsExp3 := []*types.StaticProperty{
		types.NewStaticProperty("County", types.S),
		types.NewStaticProperty("PostCode", types.N)}
	extendCoreSchemaTypeExp := types.Or(types.NewObject(extendCoreStaticPropsExp3, nil),
		types.Or(
			types.NewObject(extendCoreStaticPropsExp1, nil),
			types.NewObject(extendCoreStaticPropsExp2, nil)))

	// Test 2: anyOf is inside the core schema
	insideCoreObjType := types.NewObject([]*types.StaticProperty{
		types.NewStaticProperty("accessMe", types.S)}, nil)
	insideCoreAnyType := types.Or(insideCoreObjType, types.N)
	insideCoreStaticProps := []*types.StaticProperty{
		types.NewStaticProperty("AddressLine", types.S),
		types.NewStaticProperty("RandomInfo", insideCoreAnyType)}
	insideCoreTypeExp := types.NewObject(insideCoreStaticProps, nil)

	// Test 4: anyOf in an array
	arrayObjType1 := types.NewObject([]*types.StaticProperty{
		types.NewStaticProperty("age", types.N),
		types.NewStaticProperty("name", types.S)}, nil)
	arrayObjType2 := types.NewObject([]*types.StaticProperty{
		types.NewStaticProperty("personality", types.S),
		types.NewStaticProperty("nickname", types.S)}, nil)
	arrayOrType := types.Or(arrayObjType1, arrayObjType2)
	arrayArrayType := types.NewArray(nil, arrayOrType)
	arrayObjtype := types.NewObject([]*types.StaticProperty{
		types.NewStaticProperty("familyMembers", arrayArrayType)}, nil)

	// Test 5: anyOf array has items but not specified
	arrayMissing1 := types.NewArray([]types.Type{
		types.N, types.S}, nil)
	arrayMissing2 := types.NewArray([]types.Type{types.N}, nil)
	arrayMissingType := types.Or(arrayMissing1, arrayMissing2)

	// Test 6: anyOf Parent variation schema
	anyOfParentVar1 := types.NewObject([]*types.StaticProperty{
		types.NewStaticProperty("State", types.S),
		types.NewStaticProperty("ZipCode", types.S)}, nil)
	anyOfParentVar2 := types.NewObject([]*types.StaticProperty{
		types.NewStaticProperty("County", types.S),
		types.NewStaticProperty("PostCode", types.S)}, nil)
	anyOfParentVarType := types.Or(anyOfParentVar1, anyOfParentVar2)

	tests := []struct {
		note     string
		schema   string
		expected types.Type
	}{
		{
			note:     "anyOf extend core schema",
			schema:   anyOfExtendCoreSchema,
			expected: extendCoreSchemaTypeExp,
		},
		{
			note:     "anyOf inside core schema",
			schema:   anyOfInsideCoreSchema,
			expected: insideCoreTypeExp,
		},
		{
			note:     "anyOf object missing type",
			schema:   anyOfObjectMissing,
			expected: extendCoreSchemaTypeExp,
		},
		{
			note:     "anyOf of an array",
			schema:   anyOfArraySchema,
			expected: arrayObjtype,
		},
		{
			note:     "anyOf array missing type",
			schema:   anyOfArrayMissing,
			expected: arrayMissingType,
		},
		{
			note:     "anyOf as parent",
			schema:   anyOfSchemaParentVariation,
			expected: anyOfParentVarType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			testParseSchema(t, tc.schema, tc.expected, nil)
		})
	}
}

func kubeSchemaServer(t *testing.T) *httptest.Server {
	t.Helper()
	bs, err := os.ReadFile("testdata/_definitions.json")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(bs)
		if err != nil {
			panic(err)
		}
	}))
	return ts
}

func TestCompilerCheckTypesWithSchema(t *testing.T) {
	c := NewCompiler()
	var schema interface{}
	err := util.Unmarshal([]byte(objectSchema), &schema)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, schema)
	c.WithSchemas(schemaSet)
	compileStages(c, c.checkTypes)
	assertNotFailed(t, c)
}

func TestCompilerCheckTypesWithRegexPatternInSchema(t *testing.T) {
	c := NewCompiler()
	var schema interface{}
	// Negative lookahead is not supported in the Go regex dialect, but this is still a valid
	// JSON schema. Since we don't rely on the "pattern" attribute for type checking, ensure
	// that this still works (by being ignored)
	err := util.Unmarshal([]byte(`{
		"properties": {
			"name": {
				"pattern": "^(?!testing:.*)[a-z]+$",
				"type": "string"
			}
		}
	}`), &schema)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, schema)
	c.WithSchemas(schemaSet)
	compileStages(c, c.checkTypes)
	assertNotFailed(t, c)
}

func TestCompilerCheckTypesWithAllOfSchema(t *testing.T) {

	tests := []struct {
		note          string
		schema        string
		expectedError error
	}{
		{
			note:          "allOf with mergeable Object types in schema",
			schema:        allOfObjectSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array types in schema",
			schema:        allOfArraySchema,
			expectedError: nil,
		},
		{
			note:          "allOf without a parent schema",
			schema:        allOfSchemaParentVariation,
			expectedError: nil,
		},
		{
			note:          "allOf with empty schema",
			schema:        emptySchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array of Object types in schema",
			schema:        allOfArrayOfObjects,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Object types in schema with type declaration missing",
			schema:        allOfObjectMissing,
			expectedError: nil,
		},
		{
			note:          "allOf with Array of mergeable different types in schema",
			schema:        allOfArrayDifTypes,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Object containing Array types in schema",
			schema:        allOfArrayInsideObject,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array types in schema with type declaration missing",
			schema:        allOfArrayMissing,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable types inside of core schema",
			schema:        allOfInsideCoreSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable String types in schema",
			schema:        allOfStringSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Integer types in schema",
			schema:        allOfIntegerSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Boolean types in schema",
			schema:        allOfBooleanSchema,
			expectedError: nil,
		},
		{
			note:          "allOf with mergeable Array types with uneven numbers of items",
			schema:        allOfSchemaWithUnevenArray,
			expectedError: nil,
		},
		{
			note:          "allOf schema with unmergeable Array of Arrays",
			schema:        allOfArrayOfArrays,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf schema with Array and Object types as siblings",
			schema:        allOfObjectAndArray,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf schema with Array type that contains different unmergeable types",
			schema:        allOfArrayDifTypesWithError,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf schema with different unmergeable types",
			schema:        allOfTypeErrorSchema,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf unmergeable schema with different parent and items types",
			schema:        allOfSchemaWithParentError,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
		{
			note:          "allOf schema of Array type with uneven numbers of items to merge",
			schema:        allOfSchemaWithUnevenArray,
			expectedError: nil,
		},
		{
			note:          "allOf schema with unmergeable types String and Boolean",
			schema:        allOfStringSchemaWithError,
			expectedError: fmt.Errorf("unable to merge these schemas"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			var schema interface{}
			err := util.Unmarshal([]byte(tc.schema), &schema)
			if err != nil {
				t.Fatal("Unexpected error:", err)
			}
			schemaSet := NewSchemaSet()
			schemaSet.Put(SchemaRootRef, schema)
			c.WithSchemas(schemaSet)
			compileStages(c, c.checkTypes)
			if tc.expectedError != nil {
				if errors.Is(c.Errors, tc.expectedError) {
					t.Fatal("Unexpected error:", err)
				}
			} else {
				assertNotFailed(t, c)
			}
		})
	}
}

func TestWithSchema(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, objectSchema)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("WithSchema did not set the schema correctly in the compiler")
	}
}

func TestAnyOfObjectSchema1(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfExtendCoreSchema)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an object type schema with anyOf outside core schema")
	}
}

func TestAnyOfObjectSchema2(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfInsideCoreSchema)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an object type schema with anyOf inside core schema")
	}
}

func TestAnyOfArraySchema(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfArraySchema)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an array type schema with anyOf")
	}
}

func TestAnyOfObjectMissing(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfObjectMissing)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an object type schema with anyOf where one of the props did not explicitly claim type")
	}
}

func TestAnyOfArrayMissing(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, anyOfArrayMissing)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an array type schema with anyOf where items are inside anyOf")
	}
}

func TestRecursiveSchema(t *testing.T) {
	c := NewCompiler()
	schemaSet := NewSchemaSet()
	schemaSet.Put(SchemaRootRef, recursiveElements)
	c.WithSchemas(schemaSet)
	if c.schemaSet == nil {
		t.Fatalf("Did not correctly compile an object schema with recursive elements")
	}
}

const objectSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"required": [
		"foo",
		"b"
	],
	"properties": {
		"foo": {
			"$id": "#/properties/foo",
			"type": "string",
			"title": "The foo schema",
			"description": "An explanation about the purpose of this instance."
		},
		"b": {
			"$id": "#/properties/b",
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"additionalItems": false,
			"items": {
				"$id": "#/properties/b/items",
				"type": "object",
				"title": "The items schema",
				"description": "An explanation about the purpose of this instance.",
				"required": [
					"a",
					"b",
					"c"
				],
				"properties": {
					"a": {
						"$id": "#/properties/b/items/properties/a",
						"type": "integer",
						"title": "The a schema",
						"description": "An explanation about the purpose of this instance."
					},
					"b": {
						"$id": "#/properties/b/items/properties/b",
						"type": "array",
						"title": "The b schema",
						"description": "An explanation about the purpose of this instance.",
						"additionalItems": false,
						"items": {
							"$id": "#/properties/b/items/properties/b/items",
							"type": "integer",
							"title": "The items schema",
							"description": "An explanation about the purpose of this instance."
						}
					},
					"c": {
						"$id": "#/properties/b/items/properties/c",
						"type": "null",
						"title": "The c schema",
						"description": "An explanation about the purpose of this instance."
					}
				},
				"additionalProperties": false
			}
		}
	},
	"additionalProperties": false
}`

const arrayNoItemsSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"required": [
		"b"
	],
	"properties": {
		"b": {
			"$id": "#/properties/b",
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"additionalItems": true
		}
	},
	"additionalProperties": false
}`

const noChildrenObjectSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"additionalProperties": true
}`

const untypedFieldObjectSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"required": [
		"foo"
	],
	"properties": {
		"foo": {
			"$id": "#/properties/foo"
		}
	},
	"additionalProperties": false
}`

const booleanSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema",
	"$id": "http://example.com/example.json",
	"type": "object",
	"title": "The root schema",
	"description": "The root schema comprises the entire JSON document.",
	"required": [
		"a"
	],
	"properties": {
		"a": {
			"$id": "#/properties/foo",
			"type": "boolean",
			"title": "The foo schema",
			"description": "An explanation about the purpose of this instance."
		}
	},
	"additionalProperties": false
}`

const refSchema = `
{
    "description": "Pod is a collection of containers that can run on a host. This resource is created by clients and scheduled onto hosts.",
	"type": "object",
	"properties": {
      "apiVersion": {
        "description": "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
        "type": [
          "string",
          "null"
        ]
	  },

      "kind": {
        "description": "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": [
          "string",
          "null"
        ],
        "enum": [
          "Pod"
        ]
      },
      "metadata": {
        "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
        "description": "Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata"
	  }
	}
}
`
const podSchema = `
{
    "description": "Pod is a collection of containers that can run on a host. This resource is created by clients and scheduled onto hosts.",
    "properties": {
      "apiVersion": {
        "description": "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
        "type": [
          "string",
          "null"
        ]
      },
      "kind": {
        "description": "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
        "type": [
          "string",
          "null"
        ],
        "enum": [
          "Pod"
        ]
      },
      "metadata": {
        "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
        "description": "Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata"
      },
      "spec": {
        "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.api.core.v1.PodSpec",
        "description": "Specification of the desired behavior of the pod. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status"
      },
      "status": {
        "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.api.core.v1.PodStatus",
        "description": "Most recently observed status of the pod. This data may not be up to date. Populated by the system. Read-only. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status"
      }
    },
    "type": "object",
    "x-kubernetes-group-version-kind": [
      {
        "group": "",
        "kind": "Pod",
        "version": "v1"
      }
    ],
    "$schema": "http://json-schema.org/schema#"
  }`

const anyOfArraySchema = `{
	"type": "object",
	"properties": {
		"familyMembers": {
			"type": "array",
			"items": {
				"anyOf": [
					{
						"type": "object",
						"properties": {
							"age": { "type": "integer" },
							"name": {"type": "string"}
						}
					},{
						"type": "object",
						"properties": {
							"personality": { "type": "string" },
							"nickname": { "type": "string"  }
						}
					}
				]
			}
		}
	}
}`

const anyOfExtendCoreSchema = `{
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" }
	},
	"anyOf": [
		{
			"type": "object",
			"properties": {
				"State":   { "type": "string" },
				"ZipCode": { "type": "string" }
			}
		},
		{
			"type": "object",
			"properties": {
				"County":   { "type": "string" },
				"PostCode": { "type": "integer" }
			}
		}
	]
}`

const allOfObjectSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "type": "object",
    "title": "My schema",
    "properties": {
        "AddressLine1": { "type": "string" },
        "AddressLine2": { "type": "string" },
        "City":         { "type": "string" }
    },
    "allOf": [
        {
            "type": "object",
            "properties": {
                "State":   { "type": "string" },
                "ZipCode": { "type": "string" }
            },
        },
        {
            "type": "object",
            "properties": {
                "County":   { "type": "string" },
                "PostCode": { "type": "string" }
            },
        }
    ]
}`

const allOfArraySchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"items": {
		"type": "integer",
		"title": "The items schema",
		"description": "An explanation about the purpose of this instance."
	},
	"allOf": [
		{
		"type": "array",
		"title": "The b schema",
		"description": "An explanation about the purpose of this instance.",
		"items": {
			"type": "integer",
			"title": "The items schema",
			"description": "An explanation about the purpose of this instance."
		}
		},
		{
		"type": "array",
		"title": "The b schema",
		"description": "An explanation about the purpose of this instance.",
		"items": {
			"type": "integer",
			"title": "The items schema",
			"description": "An explanation about the purpose of this instance."
		}
		}
	]
}`

const allOfSchemaParentVariation = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "allOf": [
        {
            "type": "object",
            "properties": {
                "State":   { "type": "string" },
                "ZipCode": { "type": "string" }
            },
        },
        {
            "type": "object",
            "properties": {
                "County":   { "type": "string" },
                "PostCode": { "type": "string" }
            },
        }
    ]
}`

const emptySchema = `{
	"allof" : []
 }`

const allOfArrayOfArrays = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"items": {
		"type": "array",
		"title": "The items schema",
		"description": "An explanation about the purpose of this instance.",
		"items": {
			"type": "integer",
			"title": "The items schema",
			"description": "An explanation about the purpose of this instance."
		}
	},
	"allOf": [{
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"items": {
				"type": "array",
				"title": "The items schema",
				"description": "An explanation about the purpose of this instance.",
				"items": {
					"type": "integer",
					"title": "The items schema",
					"description": "An explanation about the purpose of this instance."
				}
			}
		},
		{
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"items": {
				"type": "integer",
				"title": "The items schema",
				"description": "An explanation about the purpose of this instance."
			}
		}
	]
}`

const anyOfInsideCoreSchema = ` {
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" },
		"RandomInfo": {
			"anyOf": [
				{ "type": "object",
				  "properties": {
					  "accessMe": {"type": "string"}
				  }
				},
				{ "type": "number", "minimum": 0 }
			  ]
		}
	}
}`

const anyOfObjectMissing = `{
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" }
	},
	"anyOf": [
		{
			"type": "object",
			"properties": {
				"State":   { "type": "string" },
				"ZipCode": { "type": "string" }
			}
		},
		{
			"properties": {
				"County":   { "type": "string" },
				"PostCode": { "type": "integer" }
			}
		}
	]
}`

const allOfArrayOfObjects = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"items": {
		"type": "object",
		"title": "The items schema",
		"description": "An explanation about the purpose of this instance.",
		"properties": {
			"State": {
				"type": "string"
			},
			"ZipCode": {
				"type": "string"
			}
		},
		"allOf": [{
				"type": "object",
				"title": "The b schema",
				"description": "An explanation about the purpose of this instance.",
				"properties": {
					"County": {
						"type": "string"
					},
					"PostCode": {
						"type": "string"
					}
				}
			},
			{
				"type": "object",
				"title": "The b schema",
				"description": "An explanation about the purpose of this instance.",
				"properties": {
					"Street": {
						"type": "string"
					},
					"House": {
						"type": "string"
					}
				}
			}
		]
	}
}`

const allOfObjectAndArray = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "object",
	"title": "My schema",
	"properties": {
		"AddressLine1": {
			"type": "string"
		},
		"AddressLine2": {
			"type": "string"
		},
		"City": {
			"type": "string"
		}
	},
	"allOf": [{
			"type": "object",
			"properties": {
				"State": {
					"type": "string"
				},
				"ZipCode": {
					"type": "string"
				}
			}
		},
		{
			"type": "array",
			"title": "The b schema",
			"description": "An explanation about the purpose of this instance.",
			"items": {
				"type": "integer",
				"title": "The items schema",
				"description": "An explanation about the purpose of this instance."
			}
		}
	]
}`

const allOfObjectMissing = `{
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" }
	},
	"allOf": [
		{
			"type": "object",
			"properties": {
				"State":   { "type": "string" },
				"ZipCode": { "type": "string" }
			}
		},
		{
			"properties": {
				"County":   { "type": "string" },
				"PostCode": { "type": "integer" }
			}
		}
	]
}`

const allOfArrayDifTypes = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "array",
			"items": [{
					"type": "string"
				},
				{
					"type": "integer"
				}
			]
		},
		{
			"type": "array",
			"items": [{
					"type": "string"
				},
				{
					"type": "integer"
				}
			]
		}
	]
}`

const allOfArrayInsideObject = `{
	"type": "object",
	"properties": {
		"familyMembers": {
			"type": "array",
			"items": {
				"allOf": [{
					"type": "object",
					"properties": {
						"age": {
							"type": "integer"
						},
						"name": {
							"type": "string"
						}
					}
				}, {
					"type": "object",
					"properties": {
						"personality": {
							"type": "string"
						},
						"nickname": {
							"type": "string"
						}
					}
				}]
			}
		}
	}
}`

const anyOfArrayMissing = `{
	"type": "array",
	"anyOf": [
		{
			"items": [
				{"type": "number"},
				{"type": "string"}]
            },
		{	"items": [
				{"type": "integer"}]
		}
	]
}`

const allOfArrayMissing = `{
	"type": "array",
	"allOf": [{
			"items": [{
					"type": "integer"
				},
				{
					"type": "integer"
				}
			]
		},
		{
			"items": [{
				"type": "integer"
			}]
		}
	]
}`

const anyOfSchemaParentVariation = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "anyOf": [
        {
            "type": "object",
            "properties": {
                "State":   { "type": "string" },
                "ZipCode": { "type": "string" }
            },
        },
        {
            "type": "object",
            "properties": {
                "County":   { "type": "string" },
                "PostCode": { "type": "string" }
            },
        }
    ]
	}
}`

const allOfInsideCoreSchema = `{
	"type": "object",
	"properties": {
		"AddressLine": { "type": "string" },
		"RandomInfo": {
			"allOf": [
				{ "type": "object",
				  "properties": {
					  "accessMe": {"type": "string"}
				  }
				},
				{ "type": "object",
					"properties": {
						"accessYou": {"type": "string"}
					}}
			  ]
		}
	}
}`

const allOfArrayDifTypesWithError = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "array",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "array",
			"items": [{
					"type": "string"
				},
				{
					"type": "integer"
				}
			]
		},
		{
			"type": "array",
			"items": [{
					"type": "boolean"
				},
				{
					"type": "integer"
				}
			]
		}
	]
}`

const allOfStringSchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "string",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "string",
		},
		{
			"type": "string",
		}
	]
}`

const allOfIntegerSchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "integer",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "integer",
		},
		{
			"type": "integer",
		}
	]
}`

const allOfBooleanSchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "boolean",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "boolean",
		},
		{
			"type": "boolean",
		}
	]
}`

const allOfTypeErrorSchema = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "string",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "string",
		},
		{
			"type": "integer",
		}
	]
}`

const allOfStringSchemaWithError = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "string",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "string",
		},
		{
			"type": "string",
		},
		{
			"type": "boolean",
		}
	]
}`

const allOfSchemaWithParentError = `{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"type": "string",
	"title": "The b schema",
	"description": "An explanation about the purpose of this instance.",
	"allOf": [{
			"type": "integer",
		},
		{
			"type": "integer",
		}
	]
}`

const allOfSchemaWithUnevenArray = `{
	"type": "array",
	"allOf": [{
			"items": [{
					"type": "integer"
				},
				{
					"type": "integer"
				}
			]
		},
		{
			"items": [{
				"type": "integer"
			},
			{
				"type": "integer"
			},
			{
				"type": "string"
			}]
		}
	]
}`

const recursiveElements = `{
  "type": "object",
  "properties": {
    "Something": {
      "$ref": "#/$defs/X"
    }
  },
  "$defs": {
    "X": {
      "type": "object",
      "properties": {
        "Y": {
          "$ref": "#/$defs/Y"
        }
      }
    },
    "Y": {
      "type": "object",
      "properties": {
        "X": {
          "$ref": "#/$defs/X"
        }
      }
    }
  }
}
`
