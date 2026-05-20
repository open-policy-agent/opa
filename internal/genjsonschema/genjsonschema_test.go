package genjsonschema

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	bs, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(bs)
}

func TestOrderedMapPreservesInsertionOrder(t *testing.T) {
	m := OrderedMap{
		{"z", 1},
		{"a", 2},
		{"m", 3},
	}
	got := mustMarshal(t, m)
	want := `{"z":1,"a":2,"m":3}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMapBuildsOrderedMap(t *testing.T) {
	got := mustMarshal(t, Map("z", 1, "a", 2, "m", 3))
	want := `{"z":1,"a":2,"m":3}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMapPanicsOnOddArgs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	args := []any{"z", 1, "a"}
	_ = Map(args...) //nolint:staticcheck // SA5012: deliberately odd to test the panic path
}

func TestMapPanicsOnNonStringKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	args := []any{42, "value"}
	_ = Map(args...)
}

func TestParseJSONTag(t *testing.T) {
	cases := []struct {
		tag, fieldName, wantName string
		wantOmit                 bool
	}{
		{"", "Foo", "Foo", false},
		{"foo", "Foo", "foo", false},
		{",omitempty", "Foo", "Foo", true},
		{"foo,omitempty", "Foo", "foo", true},
		{"foo,string,omitempty", "Foo", "foo", true},
		{"-", "Foo", "-", false},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			name, opts := parseJSONTag(tc.tag, tc.fieldName)
			if name != tc.wantName || opts.omitEmpty != tc.wantOmit {
				t.Fatalf("got (%q, omit=%v), want (%q, omit=%v)",
					name, opts.omitEmpty, tc.wantName, tc.wantOmit)
			}
		})
	}
}

func TestMakeNullableWidensTypeKey(t *testing.T) {
	in := OrderedMap{{"type", "string"}}
	got := mustMarshal(t, MakeNullable(in))
	want := `{"type":["string","null"]}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestMakeNullableWrapsRefInOneOf(t *testing.T) {
	in := OrderedMap{{"$ref", "#/$defs/Foo"}}
	got := mustMarshal(t, MakeNullable(in))
	if !strings.Contains(got, `"oneOf"`) || !strings.Contains(got, `"$ref":"#/$defs/Foo"`) ||
		!strings.Contains(got, `"type":"null"`) {
		t.Fatalf("unexpected nullable wrap: %s", got)
	}
}

func TestMakeNullableIsIdempotent(t *testing.T) {
	cases := []struct {
		note string
		in   OrderedMap
	}{
		{
			note: "type already string slice with null",
			in:   OrderedMap{{"type", []string{"string", "null"}}},
		},
		{
			note: "type is already null",
			in:   OrderedMap{{"type", "null"}},
		},
		{
			note: "already wrapped in oneOf with null branch",
			in: OrderedMap{
				{"oneOf", []any{
					OrderedMap{{"$ref", "#/$defs/Foo"}},
					OrderedMap{{"type", "null"}},
				}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			before := mustMarshal(t, tc.in)
			after := mustMarshal(t, MakeNullable(tc.in))
			if before != after {
				t.Fatalf("MakeNullable not idempotent:\n  before: %s\n  after:  %s", before, after)
			}
			// And widen-by-applying-twice (composed) should match applying once.
			once := mustMarshal(t, MakeNullable(OrderedMap{{"type", "string"}}))
			twice := mustMarshal(t, MakeNullable(MakeNullable(OrderedMap{{"type", "string"}})))
			if once != twice {
				t.Fatalf("double-application diverges:\n  once:  %s\n  twice: %s", once, twice)
			}
		})
	}
}

func TestMakeNullableDoesNotMutateInput(t *testing.T) {
	// Widening a string type must not mutate the caller's OrderedMap.
	in := OrderedMap{{"type", "string"}}
	before := mustMarshal(t, in)
	_ = MakeNullable(in)
	if after := mustMarshal(t, in); before != after {
		t.Fatalf("string-type input was mutated:\n  before: %s\n  after:  %s", before, after)
	}

	// Widening a []string type must neither mutate the OrderedMap nor write
	// into the spare capacity of the original slice's backing array — a slice
	// with len < cap would otherwise see "null" appear at types[len:cap].
	types := make([]string, 1, 4)
	types[0] = "string"
	in2 := OrderedMap{{"type", types}}
	before2 := mustMarshal(t, in2)
	_ = MakeNullable(in2)
	if after2 := mustMarshal(t, in2); before2 != after2 {
		t.Fatalf("[]string-type input was mutated:\n  before: %s\n  after:  %s", before2, after2)
	}
	expanded := types[:cap(types)]
	for i := 1; i < len(expanded); i++ {
		if expanded[i] != "" {
			t.Fatalf("MakeNullable wrote into backing slice at index %d: %v", i, expanded)
		}
	}
}

type primitives struct {
	S string `json:"s"`
	I int    `json:"i"`
	B bool   `json:"b"`
	F float64
}

func TestReflectStructPrimitives(t *testing.T) {
	b := NewBuilder(nil)
	if _, err := b.AddStruct(reflect.TypeOf(primitives{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	// Fields are sorted alphabetically by JSON name; F has no tag so falls
	// back to the Go field name.
	want := `{"primitives":{"type":"object","properties":{"F":{"type":"number"},"b":{"type":"boolean"},"i":{"type":"integer"},"s":{"type":"string"}},"required":["F","b","i","s"],"additionalProperties":false}}`
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

type omitFields struct {
	Required string  `json:"required"`
	Optional string  `json:"optional,omitempty"`
	PtrOpt   *string `json:"ptr_opt,omitempty"`
	PtrReq   *string `json:"ptr_req"`
}

func TestOmitEmptyAndNullability(t *testing.T) {
	b := NewBuilder(nil)
	if _, err := b.AddStruct(reflect.TypeOf(omitFields{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	// PtrReq is required and nullable (pointer, not omitempty).
	// PtrOpt is omitempty so neither required nor nullable.
	// Required is required; Optional is not.
	want := `{"omitFields":{"type":"object","properties":{"optional":{"type":"string"},"ptr_opt":{"type":"string"},"ptr_req":{"type":["string","null"]},"required":{"type":"string"}},"required":["ptr_req","required"],"additionalProperties":false}}`
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

type withMaps struct {
	Counts map[string]int `json:"counts"`
	Bag    map[string]any `json:"bag"`
}

func TestMapHandling(t *testing.T) {
	b := NewBuilder(nil)
	if _, err := b.AddStruct(reflect.TypeOf(withMaps{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	if !strings.Contains(got, `"counts":{"type":["object","null"],"additionalProperties":{"type":"integer"}}`) {
		t.Fatalf("typed-value map not as expected: %s", got)
	}
	if !strings.Contains(got, `"bag":{"type":["object","null"]}`) {
		t.Fatalf("any-value map not as expected: %s", got)
	}
}

type inner struct {
	X int `json:"x"`
}
type outer struct {
	A inner  `json:"a"`
	B *inner `json:"b,omitempty"`
}

func TestNestedStructsAndPointer(t *testing.T) {
	b := NewBuilder(nil)
	if _, err := b.AddStruct(reflect.TypeOf(outer{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	if !strings.Contains(got, `"$ref":"#/$defs/inner"`) {
		t.Fatalf("expected nested ref to inner: %s", got)
	}
	if !strings.Contains(got, `"inner":{"type":"object","properties":{"x":{"type":"integer"}}`) {
		t.Fatalf("inner def missing or malformed: %s", got)
	}
}

type EmbedBase struct {
	BaseField string `json:"base_field"`
}
type embedder struct {
	EmbedBase
	Own int `json:"own"`
}

func TestEmbeddedStructFieldsArePromoted(t *testing.T) {
	b := NewBuilder(nil)
	if _, err := b.AddStruct(reflect.TypeOf(embedder{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	// Promoted field appears at top level; no separate def for EmbedBase.
	if !strings.Contains(got, `"base_field":{"type":"string"}`) {
		t.Fatalf("expected promoted base_field: %s", got)
	}
	if strings.Contains(got, `"EmbedBase"`) {
		t.Fatalf("did not expect EmbedBase to get its own def: %s", got)
	}
}

type marker interface{ marker() }

type withInterface struct {
	M marker `json:"m,omitempty"`
}

func TestResolverInterceptsTypes(t *testing.T) {
	resolver := func(_ *Builder, t reflect.Type) (any, bool, error) {
		if t == reflect.TypeOf((*marker)(nil)).Elem() {
			return OrderedMap{{"description", "opaque marker"}}, true, nil
		}
		return nil, false, nil
	}
	b := NewBuilder(resolver)
	if _, err := b.AddStruct(reflect.TypeOf(withInterface{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	if !strings.Contains(got, `"m":{"description":"opaque marker"}`) {
		t.Fatalf("resolver result not propagated: %s", got)
	}
}

type withRequiredInterface struct {
	M marker `json:"m"`
}

func TestResolverResultIsNullableWhenFieldCanBeNull(t *testing.T) {
	// When the field type can encode as JSON null and isn't omitempty, the
	// resolver's bare schema gets wrapped in a nullable form so the
	// generated schema admits null in addition to the resolved shape.
	resolver := func(_ *Builder, t reflect.Type) (any, bool, error) {
		if t == reflect.TypeOf((*marker)(nil)).Elem() {
			return OrderedMap{{"description", "opaque marker"}}, true, nil
		}
		return nil, false, nil
	}
	b := NewBuilder(resolver)
	if _, err := b.AddStruct(reflect.TypeOf(withRequiredInterface{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	if !strings.Contains(got, `"oneOf"`) || !strings.Contains(got, `"opaque marker"`) ||
		!strings.Contains(got, `"type":"null"`) {
		t.Fatalf("expected nullable wrap: %s", got)
	}
}

func TestInterfaceWithoutResolverErrors(t *testing.T) {
	b := NewBuilder(nil)
	_, err := b.AddStruct(reflect.TypeOf(withInterface{}))
	if err == nil {
		t.Fatal("expected error for unresolved non-empty interface")
	}
}

func TestResolverHandledWithNilSchemaErrors(t *testing.T) {
	// A resolver that reports handled=true must return a non-nil schema;
	// otherwise the field would marshal as JSON null, silently corrupting
	// output.
	resolver := func(_ *Builder, t reflect.Type) (any, bool, error) {
		if t == reflect.TypeOf((*marker)(nil)).Elem() {
			return nil, true, nil
		}
		return nil, false, nil
	}
	b := NewBuilder(resolver)
	_, err := b.AddStruct(reflect.TypeOf(withInterface{}))
	if err == nil || !strings.Contains(err.Error(), "nil schema") {
		t.Fatalf("expected nil-schema error, got: %v", err)
	}
}

func TestAddNamedDefRejectsCollision(t *testing.T) {
	b := NewBuilder(nil)
	if _, err := b.AddNamedDef("Foo", OrderedMap{{"type", "string"}}); err != nil {
		t.Fatalf("first AddNamedDef: %v", err)
	}
	if _, err := b.AddNamedDef("Foo", OrderedMap{{"type", "integer"}}); err == nil {
		t.Fatal("expected error on duplicate AddNamedDef")
	}
	// Reserved-but-not-yet-set name also collides.
	b.Reserve("Bar")
	if _, err := b.AddNamedDef("Bar", OrderedMap{{"type", "string"}}); err == nil {
		t.Fatal("expected error on AddNamedDef colliding with Reserve")
	}
}

func TestReserveAndSetDefBreakCycles(t *testing.T) {
	b := NewBuilder(nil)
	const name = "RecursiveBranch"
	if !b.Reserve(name) {
		t.Fatal("Reserve returned false on first call")
	}
	if b.Reserve(name) {
		t.Fatal("Reserve returned true on second call")
	}
	if !b.HasDef(name) {
		t.Fatal("HasDef false after Reserve")
	}
	b.SetDef(name, OrderedMap{{"type", "string"}})
	got := mustMarshal(t, b.DefsOrdered())
	if !strings.Contains(got, `"RecursiveBranch":{"type":"string"}`) {
		t.Fatalf("def not stored: %s", got)
	}
}

type unsupportedKind struct {
	Ch chan int `json:"ch"`
}

func TestUnsupportedKindReportsFieldPath(t *testing.T) {
	b := NewBuilder(nil)
	_, err := b.AddStruct(reflect.TypeOf(unsupportedKind{}))
	if err == nil || !strings.Contains(err.Error(), "unsupportedKind.Ch") {
		t.Fatalf("expected error mentioning field path, got: %v", err)
	}
}

type intKeyMap struct {
	M map[int]string `json:"m"`
}

func TestNonStringMapKeyErrors(t *testing.T) {
	b := NewBuilder(nil)
	_, err := b.AddStruct(reflect.TypeOf(intKeyMap{}))
	if err == nil || !strings.Contains(err.Error(), "map key") {
		t.Fatalf("expected map-key error, got: %v", err)
	}
}

func TestAllowAdditionalPropertiesSkipsClosedClause(t *testing.T) {
	b := NewBuilder(nil)
	b.AllowAdditionalProperties(reflect.TypeOf(primitives{}))
	if _, err := b.AddStruct(reflect.TypeOf(primitives{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	if strings.Contains(got, `"additionalProperties":false`) {
		t.Fatalf("did not expect additionalProperties:false in opt-out type:\n%s", got)
	}
	// Sanity check: opting in by pointer type also works.
	b2 := NewBuilder(nil)
	b2.AllowAdditionalProperties(reflect.TypeOf(&primitives{}))
	if _, err := b2.AddStruct(reflect.TypeOf(primitives{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got2 := mustMarshal(t, b2.DefsOrdered())
	if strings.Contains(got2, `"additionalProperties":false`) {
		t.Fatalf("did not expect additionalProperties:false when opted in via pointer type:\n%s", got2)
	}
}

func TestAllowAdditionalPropertiesIsPerType(t *testing.T) {
	// Opting `outer` in must not loosen the constraint on its nested
	// `inner` def — additionalProperties:false is the right default for
	// sub-records.
	b := NewBuilder(nil)
	b.AllowAdditionalProperties(reflect.TypeOf(outer{}))
	if _, err := b.AddStruct(reflect.TypeOf(outer{})); err != nil {
		t.Fatalf("AddStruct: %v", err)
	}
	got := mustMarshal(t, b.DefsOrdered())
	if !strings.Contains(got, `"inner":{"type":"object","properties":{"x":{"type":"integer"}},"required":["x"],"additionalProperties":false}`) {
		t.Fatalf("expected inner to remain strict: %s", got)
	}
}
