package ast

import (
	"testing"

	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

func testParseSchema(t *testing.T, schema string, expectedType types.Type) {
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
	//Expected type is: object<b: array<object<a: number, b: array<number>, c: any>>, foo: string>
	innerObjectStaticProps := []*types.StaticProperty{}
	innerObjectStaticProps = append(innerObjectStaticProps, &types.StaticProperty{Key: "a", Value: types.N})
	innerObjectStaticProps = append(innerObjectStaticProps, &types.StaticProperty{Key: "b", Value: types.NewArray([]types.Type{types.N}, nil)})
	innerObjectStaticProps = append(innerObjectStaticProps, &types.StaticProperty{Key: "c", Value: types.A})
	innerObjectType := types.NewObject(innerObjectStaticProps, nil)

	staticProps := []*types.StaticProperty{}
	staticProps = append(staticProps, &types.StaticProperty{Key: "b", Value: types.NewArray([]types.Type{innerObjectType}, nil)})
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
	if newtype.String() != "object<apiVersion: string, kind: string, metadata: object<annotations: object[any: any], clusterName: string, creationTimestamp: string, deletionGracePeriodSeconds: number, deletionTimestamp: string, finalizers: array<string>, generateName: string, generation: number, initializers: object<pending: array<object<name: string>>, result: object<apiVersion: string, code: number, details: object<causes: array<object<field: string, message: string, reason: string>>, group: string, kind: string, name: string, retryAfterSeconds: number, uid: string>, kind: string, message: string, metadata: object<continue: string, resourceVersion: string, selfLink: string>, reason: string, status: string>>, labels: object[any: any], managedFields: array<object<apiVersion: string, fields: object[any: any], manager: string, operation: string, time: string>>, name: string, namespace: string, ownerReferences: array<object<apiVersion: string, blockOwnerDeletion: boolean, controller: boolean, kind: string, name: string, uid: string>>, resourceVersion: string, selfLink: string, uid: string>>" {
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
