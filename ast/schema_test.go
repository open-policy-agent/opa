package ast

import (
	"testing"

	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

func testParseSchema(t *testing.T, schema string, expectedType types.Type) {
	t.Helper()

	var sch interface{}
	err := util.Unmarshal([]byte(schema), &sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	newtype, err := loadSchema(sch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newtype == nil {
		t.Fatalf("parseSchema returned nil type")
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
	testParseSchema(t, objectSchema, expectedType)
}

func TestSetTypesWithSchemaRef(t *testing.T) {
	var sch interface{}
	err := util.Unmarshal([]byte(refSchema), &sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	newtype, err := loadSchema(sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if newtype == nil {
		t.Fatalf("parseSchema returned nil type")
	}
	if newtype.String() != "object<apiVersion: string, kind: string, metadata: object<annotations: object[any: any], clusterName: string, creationTimestamp: string, deletionGracePeriodSeconds: number, deletionTimestamp: string, finalizers: array[string], generateName: string, generation: number, initializers: object<pending: array[object<name: string>], result: object<apiVersion: string, code: number, details: object<causes: array[object<field: string, message: string, reason: string>], group: string, kind: string, name: string, retryAfterSeconds: number, uid: string>, kind: string, message: string, metadata: object<continue: string, resourceVersion: string, selfLink: string>, reason: string, status: string>>, labels: object[any: any], managedFields: array[object<apiVersion: string, fields: object[any: any], manager: string, operation: string, time: string>], name: string, namespace: string, ownerReferences: array[object<apiVersion: string, blockOwnerDeletion: boolean, controller: boolean, kind: string, name: string, uid: string>], resourceVersion: string, selfLink: string, uid: string>>" {
		t.Fatalf("parseSchema returned an incorrect type: %s", newtype.String())
	}
}

func TestSetTypesWithPodSchema(t *testing.T) {
	var sch interface{}
	err := util.Unmarshal([]byte(podSchema), &sch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	newtype, err := loadSchema(sch)
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

func TestParseSchemaUntypedField(t *testing.T) {
	//Expected type is: object<foo: any>
	staticProps := []*types.StaticProperty{}
	staticProps = append(staticProps, &types.StaticProperty{Key: "foo", Value: types.A})
	expectedType := types.NewObject(staticProps, nil)
	testParseSchema(t, untypedFieldObjectSchema, expectedType)
}

func TestParseSchemaNoChildren(t *testing.T) {
	//Expected type is: object[any: any]
	expectedType := types.NewObject(nil, &types.DynamicProperty{Key: types.A, Value: types.A})
	testParseSchema(t, noChildrenObjectSchema, expectedType)
}

func TestParseSchemaArrayNoItems(t *testing.T) {
	//Expected type is: object<b: array[any]>
	staticProps := []*types.StaticProperty{}
	staticProps = append(staticProps, &types.StaticProperty{Key: "b", Value: types.NewArray(nil, types.A)})
	expectedType := types.NewObject(staticProps, nil)
	testParseSchema(t, arrayNoItemsSchema, expectedType)
}

func TestParseSchemaBooleanField(t *testing.T) {
	//Expected type is: object<a: boolean>
	staticProps := []*types.StaticProperty{}
	staticProps = append(staticProps, &types.StaticProperty{Key: "a", Value: types.B})
	expectedType := types.NewObject(staticProps, nil)
	testParseSchema(t, booleanSchema, expectedType)
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
			testParseSchema(t, tc.schema, tc.exp)
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
	jsonSchema, _ := compileSchema(sch)
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
	jsonSchema, err := compileSchema(sch)
	if err != nil {
		t.Fatalf("Unable to compile schema: %v", err)
	}
	newtype, err := parseSchema(jsonSchema) // Did not pass the subschema
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
			testParseSchema(t, tc.schema, tc.expected)
		})
	}
}
