// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package protoschemacheck verifies that a hand-authored .proto file
// stays consistent with the Go types it mirrors. Powers the proto
// consistency tests for v1/bundle/manifest.proto and v1/ir/plan.proto.
package protoschemacheck

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bufbuild/protocompile"
	"github.com/open-policy-agent/opa/v1/util"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// loadTimeout caps how long we'll wait for protocompile to parse the
// proto file so a pathological input fails fast.
const loadTimeout = 30 * time.Second

// Spec describes a consistency check between a .proto file and a set of
// Go types.
type Spec struct {
	// ProtoPath is the .proto file relative to the test's working dir.
	ProtoPath string
	// ImportPaths are extra search dirs for imports; "." is added by default.
	ImportPaths []string
	// Messages enumerates proto messages to validate. Every message
	// declared in the .proto (including nested) must appear here or in
	// OpaqueMessages.
	Messages []MessageSpec
	// OpaqueMessages names polymorphic-union envelope messages with no
	// direct Go counterpart (content is checked via Oneofs). Every entry
	// must pair with an OneofSpec.
	OpaqueMessages []string
	// Oneofs enumerates polymorphic-union checks.
	Oneofs []OneofSpec
}

// MessageSpec asserts that proto message Name matches GoType.
type MessageSpec struct {
	Name string
	// GoType is the Go struct compared against the proto message.
	// Anonymous (embedded) struct fields are flattened recursively
	// (mirroring encoding/json) unless listed in SkipEmbeddedTypes.
	GoType reflect.Type
	// SkipEmbeddedTypes opts out specific embedded types from flattening
	// (their fields are validated on a different proto message — see
	// the IR plan schema where Stmt bodies embed Location but the proto
	// promotes Location's fields onto the Stmt envelope).
	SkipEmbeddedTypes []reflect.Type
	// FieldNameOverride remaps Go-side JSON name → proto field name
	// where the default mapping doesn't apply (e.g. ir.MakeNumberRefStmt's
	// Index Go field maps to proto field "index").
	FieldNameOverride map[string]string
	// OpaqueProtoFields names proto fields whose Go counterpart isn't
	// reflectable into proto (e.g. types.Function fields modeled as
	// google.protobuf.Struct). The check requires the Go field to exist
	// with a structural type — opaque is "no type check feasible", not
	// "no Go field expected".
	OpaqueProtoFields []string
	// SkipGoFields names Go fields (by Go field name, not JSON name)
	// that have no proto counterpart by design — e.g. ir.BuiltinFunc.Decl
	// is intentionally absent from the proto because consumers consult
	// their own registry for builtin signatures.
	SkipGoFields []string
	// SkipProtoFields names proto fields with no Go counterpart by design —
	// wire-form bookkeeping that the Go side carries differently. For
	// example, bundle.Manifest.roots_set distinguishes nil from explicit-
	// empty on the wire; the Go side already carries that as Roots *[]string.
	SkipProtoFields []string
}

// OneofSpec asserts that the named oneof on MessageName has exactly the
// listed cases with matching Go-side types.
type OneofSpec struct {
	MessageName string
	OneofName   string
	// DiscriminatorToCase maps each JSON-discriminator (runtime kind
	// name) to the proto oneof case name.
	DiscriminatorToCase map[string]string
	// DiscriminatorToGoType maps each discriminator to the Go type
	// implementing the union. Must have the same key set as
	// DiscriminatorToCase. Struct types check by name; scalar types
	// check by reflect.Kind.
	DiscriminatorToGoType map[string]reflect.Type
}

// Run executes the spec, reporting every drift via t.Errorf.
func Run(t *testing.T, spec Spec) {
	t.Helper()
	if err := validateSpec(spec); err != nil {
		t.Fatalf("invalid spec: %v", err)
	}
	file := loadProto(t, spec.ProtoPath, spec.ImportPaths)

	declared := map[string]protoreflect.MessageDescriptor{}
	collectDeclared(file.Messages(), declared)

	covered := map[string]bool{}
	oneofParents := map[string]bool{}
	for _, o := range spec.Oneofs {
		oneofParents[o.MessageName] = true
	}
	// Collect, per message, the names of oneofs covered by an OneofSpec.
	// Fields belonging to those oneofs are validated by checkOneof and
	// must NOT be reported as orphans by checkMessage's field walk.
	coveredOneofs := map[string]map[string]bool{}
	for _, o := range spec.Oneofs {
		if coveredOneofs[o.MessageName] == nil {
			coveredOneofs[o.MessageName] = map[string]bool{}
		}
		coveredOneofs[o.MessageName][o.OneofName] = true
	}
	for _, m := range spec.Messages {
		covered[m.Name] = true
		checkMessage(t, declared, m, coveredOneofs[m.Name])
	}
	for _, name := range spec.OpaqueMessages {
		covered[name] = true
		if _, ok := declared[name]; !ok {
			t.Errorf("opaque message %q listed in spec but not present in %s", name, spec.ProtoPath)
		}
		if !oneofParents[name] {
			t.Errorf("opaque message %q has no matching OneofSpec; OpaqueMessages is for polymorphic envelopes only — add an OneofSpec or move the message into Messages", name)
		}
		if hasMessageSpec(spec, name) {
			t.Errorf("message %q is listed in both Messages and OpaqueMessages — pick one", name)
		}
	}
	for name := range declared {
		if !covered[name] {
			t.Errorf("proto message %q is declared in %s but not covered by the spec; add a MessageSpec or list it in OpaqueMessages", name, spec.ProtoPath)
		}
	}

	for _, o := range spec.Oneofs {
		checkOneof(t, declared, o)
	}
}

func validateSpec(s Spec) error {
	if s.ProtoPath == "" {
		return errors.New("ProtoPath is required")
	}
	for _, o := range s.Oneofs {
		// EqualFunc with an always-true comparator collapses to a key-set
		// equality check, ignoring the differing value types.
		eq := maps.EqualFunc(o.DiscriminatorToCase, o.DiscriminatorToGoType,
			func(string, reflect.Type) bool { return true })
		if !eq {
			return fmt.Errorf("oneof %s.%s: DiscriminatorToCase and DiscriminatorToGoType have different key sets",
				o.MessageName, o.OneofName)
		}
	}
	return nil
}

func hasMessageSpec(s Spec, name string) bool {
	for _, m := range s.Messages {
		if m.Name == name {
			return true
		}
	}
	return false
}

func collectDeclared(msgs protoreflect.MessageDescriptors, out map[string]protoreflect.MessageDescriptor) {
	for i := range msgs.Len() {
		m := msgs.Get(i)
		// Skip synthetic map-entry messages — proto generates them
		// implicitly for map<K,V> fields and they have no user-visible
		// counterpart in either the proto source or the Go types.
		if m.IsMapEntry() {
			continue
		}
		out[string(m.Name())] = m
		collectDeclared(m.Messages(), out)
	}
}

func loadProto(t *testing.T, protoPath string, importPaths []string) protoreflect.FileDescriptor {
	t.Helper()
	paths := append([]string{"."}, importPaths...)
	c := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: paths,
		}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
	defer cancel()
	files, err := c.Compile(ctx, protoPath)
	if err != nil {
		t.Fatalf("compile %s: %v", protoPath, err)
	}
	if len(files) != 1 {
		t.Fatalf("compile %s: expected 1 file, got %d", protoPath, len(files))
	}
	return files[0]
}

func checkMessage(t *testing.T, declared map[string]protoreflect.MessageDescriptor, m MessageSpec, coveredOneofs map[string]bool) {
	t.Helper()
	msg, ok := declared[m.Name]
	if !ok {
		t.Errorf("proto message %q not declared in any input file", m.Name)
		return
	}

	override := m.FieldNameOverride
	skipEmbedded := map[reflect.Type]bool{}
	for _, et := range m.SkipEmbeddedTypes {
		skipEmbedded[et] = true
	}
	skipGo := map[string]bool{}
	for _, n := range m.SkipGoFields {
		skipGo[n] = true
	}

	type expected struct {
		protoName string
		goType    reflect.Type
		goName    string
	}
	var want []expected
	visited := map[reflect.Type]bool{}
	var collect func(rt reflect.Type, qualifier string)
	collect = func(rt reflect.Type, qualifier string) {
		if rt.Kind() != reflect.Struct {
			return
		}
		if visited[rt] {
			return
		}
		visited[rt] = true
		for i := range rt.NumField() {
			f := rt.Field(i)
			if !f.IsExported() {
				continue
			}
			if skipGo[f.Name] {
				continue
			}
			if f.Anonymous {
				ft := f.Type
				for ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Struct {
					if skipEmbedded[ft] {
						continue
					}
					collect(ft, qualifier)
					continue
				}
			}
			name, ok := jsonFieldName(f)
			if !ok {
				continue
			}
			if remap, has := override[name]; has {
				name = remap
			}
			want = append(want, expected{
				protoName: name,
				goType:    f.Type,
				goName:    qualifier + "." + f.Name,
			})
		}
	}
	collect(m.GoType, m.GoType.Name())

	// Validate SkipGoFields entries point at real Go fields.
	for _, n := range m.SkipGoFields {
		if !goFieldExists(m.GoType, n) {
			t.Errorf("%s: SkipGoFields entry %q does not match any Go field", m.Name, n)
		}
	}

	opaque := map[string]bool{}
	for _, n := range m.OpaqueProtoFields {
		opaque[n] = true
		if msg.Fields().ByName(protoreflect.Name(n)) == nil {
			t.Errorf("%s: OpaqueProtoFields entry %q does not match any proto field on this message", m.Name, n)
		}
	}

	skipProto := map[string]bool{}
	for _, n := range m.SkipProtoFields {
		skipProto[n] = true
		if msg.Fields().ByName(protoreflect.Name(n)) == nil {
			t.Errorf("%s: SkipProtoFields entry %q does not match any proto field on this message", m.Name, n)
		}
	}

	// Validate FieldNameOverride entries point at real Go fields.
	for goJSONName := range override {
		if !goJSONNameExists(m.GoType, goJSONName) {
			t.Errorf("%s: FieldNameOverride key %q does not match any Go field's JSON name", m.Name, goJSONName)
		}
	}

	seen := map[string]bool{}
	for _, w := range want {
		seen[w.protoName] = true
		pf := msg.Fields().ByName(protoreflect.Name(w.protoName))
		if pf == nil {
			t.Errorf("%s: Go field %s maps to proto field %q but %s has no such field; add it with the next available field number", m.Name, w.goName, w.protoName, m.Name)
			continue
		}
		if opaque[w.protoName] {
			// Opaque means "no type check feasible" — but the Go side
			// must still be a structural type. A scalar Go field paired
			// with an opaque proto field is almost certainly a drift bug.
			switch unwrapPointer(w.goType).Kind() {
			case reflect.Bool,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64,
				reflect.String:
				t.Errorf("%s.%s (Go field %s): proto field is opaque but Go type %s is scalar; opaque is for unreflectable structural types only", m.Name, w.protoName, w.goName, w.goType)
			}
			continue
		}
		if err := checkFieldType(w.goType, pf); err != nil {
			t.Errorf("%s.%s (Go field %s): %v", m.Name, w.protoName, w.goName, err)
		}
	}

	// Find proto fields that have no Go counterpart.
	for i := range msg.Fields().Len() {
		pf := msg.Fields().Get(i)
		name := string(pf.Name())
		if seen[name] {
			continue
		}
		if opaque[name] {
			continue
		}
		if skipProto[name] {
			continue
		}
		// Fields belonging to a oneof handled by an OneofSpec are
		// validated there, not here. Skip them so the orphan check
		// doesn't double-fire.
		if oo := pf.ContainingOneof(); oo != nil && coveredOneofs[string(oo.Name())] {
			continue
		}
		t.Errorf("%s: proto field %q (number %d) has no corresponding Go field; either remove it (and add `reserved %d`) or add a matching Go field", m.Name, name, pf.Number(), pf.Number())
	}
}

func goJSONNameExists(rt reflect.Type, target string) bool {
	if rt.Kind() != reflect.Struct {
		return false
	}
	for i := range rt.NumField() {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Anonymous {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct && goJSONNameExists(ft, target) {
				return true
			}
			continue
		}
		name, ok := jsonFieldName(f)
		if !ok {
			continue
		}
		if name == target {
			return true
		}
	}
	return false
}

func goFieldExists(rt reflect.Type, target string) bool {
	if rt.Kind() != reflect.Struct {
		return false
	}
	for i := range rt.NumField() {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Name == target {
			return true
		}
		if f.Anonymous {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct && goFieldExists(ft, target) {
				return true
			}
		}
	}
	return false
}

func checkOneof(t *testing.T, declared map[string]protoreflect.MessageDescriptor, o OneofSpec) {
	t.Helper()
	msg, ok := declared[o.MessageName]
	if !ok {
		t.Errorf("oneof check: proto message %q not declared", o.MessageName)
		return
	}
	oneof := msg.Oneofs().ByName(protoreflect.Name(o.OneofName))
	if oneof == nil {
		t.Errorf("oneof check: %s has no oneof named %q", o.MessageName, o.OneofName)
		return
	}

	caseFields := map[string]protoreflect.FieldDescriptor{}
	for i := range oneof.Fields().Len() {
		f := oneof.Fields().Get(i)
		caseFields[string(f.Name())] = f
	}

	discriminators := slices.Sorted(maps.Keys(o.DiscriminatorToCase))

	expectedCases := map[string]bool{}
	for _, d := range discriminators {
		caseName := o.DiscriminatorToCase[d]
		expectedCases[caseName] = true
		f, ok := caseFields[caseName]
		if !ok {
			t.Errorf("%s.%s: discriminator %q maps to oneof case %q, but no such case exists", o.MessageName, o.OneofName, d, caseName)
			continue
		}
		goType := o.DiscriminatorToGoType[d]
		if goType == nil {
			t.Errorf("%s.%s.%s: discriminator %q has nil Go type — every discriminator must declare a concrete type so the proto kind can be checked", o.MessageName, o.OneofName, caseName, d)
			continue
		}
		gt := unwrapPointer(goType)
		gk := gt.Kind()
		if gk == reflect.Struct {
			if f.Kind() != protoreflect.MessageKind {
				t.Errorf("%s.%s.%s: discriminator %q expected message-typed oneof case but proto field is a scalar of kind %s", o.MessageName, o.OneofName, caseName, d, f.Kind())
				continue
			}
			want := gt.Name()
			got := string(f.Message().Name())
			if got != want {
				t.Errorf("%s.%s.%s: discriminator %q references Go type %s but proto case wraps message %s", o.MessageName, o.OneofName, caseName, d, want, got)
			}
			continue
		}
		// Scalar Go kind — validate proto kind compatibility.
		if err := checkScalarKind(gt, f); err != nil {
			t.Errorf("%s.%s.%s: discriminator %q: %v", o.MessageName, o.OneofName, caseName, d, err)
		}
	}

	// Sort orphan-case names for deterministic error ordering.
	var orphanCases []string
	for caseName := range caseFields {
		if !expectedCases[caseName] {
			orphanCases = append(orphanCases, caseName)
		}
	}
	for _, caseName := range util.Sorted(orphanCases) {
		f := caseFields[caseName]
		t.Errorf("%s.%s: proto case %q (number %d) has no corresponding discriminator in DiscriminatorToCase; either remove it (and `reserved %d` the number) or extend the spec", o.MessageName, o.OneofName, caseName, f.Number(), f.Number())
	}
}

// jsonFieldName returns the JSON name of f and whether it should be included
// (false for `json:"-"` fields).
func jsonFieldName(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	if tag == "" {
		return f.Name, true
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		name = f.Name
	}
	return name, true
}

// checkFieldType verifies that the Go field type goType is compatible with
// the proto field descriptor pf.
func checkFieldType(goType reflect.Type, pf protoreflect.FieldDescriptor) error {
	goType = unwrapPointer(goType)

	if pf.IsList() {
		if goType.Kind() != reflect.Slice && goType.Kind() != reflect.Array {
			return fmt.Errorf("proto field is repeated but Go type is %s", goType.Kind())
		}
		elem := unwrapPointer(goType.Elem())
		return checkScalarOrMessage(elem, pf)
	}
	if pf.IsMap() {
		if goType.Kind() != reflect.Map {
			return fmt.Errorf("proto field is a map but Go type is %s", goType.Kind())
		}
		if goType.Key().Kind() != reflect.String {
			return fmt.Errorf("proto field is a map but Go map key is %s, want string", goType.Key().Kind())
		}
		valField := pf.MapValue()
		valType := unwrapPointer(goType.Elem())
		return checkScalarOrMessage(valType, valField)
	}
	return checkScalarOrMessage(goType, pf)
}

func checkScalarOrMessage(goType reflect.Type, pf protoreflect.FieldDescriptor) error {
	goType = unwrapPointer(goType)
	if pf.Kind() == protoreflect.MessageKind || pf.Kind() == protoreflect.GroupKind {
		// Accept any Go type for message-typed proto fields. The outer
		// message check verifies field-by-field shape; the
		// Stmt/Val/Operand/Block plumbing relies on this looseness.
		return nil
	}
	return checkScalarKind(goType, pf)
}

// checkScalarKind verifies a Go type is compatible with a scalar proto kind.
// For integer kinds we enforce that the Go bit-width fits in the proto
// bit-width (so Go int64 cannot be silently mapped to proto int32). The
// platform-dependent `int`/`uint` kinds are treated as 32-bit minimum and
// accepted against either width — practical for OPA's bounded indices.
func checkScalarKind(goType reflect.Type, pf protoreflect.FieldDescriptor) error {
	switch pf.Kind() {
	case protoreflect.BoolKind:
		if goType.Kind() != reflect.Bool {
			return fmt.Errorf("proto Kind=bool but Go type is %s", goType.Kind())
		}
	case protoreflect.StringKind:
		if goType.Kind() != reflect.String {
			return fmt.Errorf("proto Kind=string but Go type is %s", goType.Kind())
		}
	case protoreflect.BytesKind:
		if !(goType.Kind() == reflect.Slice && goType.Elem().Kind() == reflect.Uint8) {
			return fmt.Errorf("proto Kind=bytes but Go type is %s", goType.Kind())
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		if !signedIntFits(goType.Kind(), pf.Kind()) {
			return fmt.Errorf("proto Kind=%s but Go type is %s (Go int64 cannot be safely narrowed)", pf.Kind(), goType.Kind())
		}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if !unsignedIntFits(goType.Kind(), pf.Kind()) {
			return fmt.Errorf("proto Kind=%s but Go type is %s (Go uint64 cannot be safely narrowed)", pf.Kind(), goType.Kind())
		}
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		if goType.Kind() != reflect.Float32 && goType.Kind() != reflect.Float64 {
			return fmt.Errorf("proto Kind=%s but Go type is %s", pf.Kind(), goType.Kind())
		}
	default:
		return fmt.Errorf("unsupported proto Kind=%s", pf.Kind())
	}
	return nil
}

// signedIntFits reports whether a Go signed integer kind fits in a proto
// signed integer kind. Go's platform-dependent `int` is treated as
// at-most 32-bit (lenient — accepted against either int32 or int64 proto
// kinds). Fixed-width kinds (int8/int16/int32/int64) are checked strictly.
func signedIntFits(goKind reflect.Kind, pfKind protoreflect.Kind) bool {
	proto64 := pfKind == protoreflect.Int64Kind || pfKind == protoreflect.Sint64Kind || pfKind == protoreflect.Sfixed64Kind
	switch goKind {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int:
		return true // fits any signed proto int kind
	case reflect.Int64:
		return proto64
	}
	return false
}

// unsignedIntFits is the unsigned counterpart of signedIntFits.
func unsignedIntFits(goKind reflect.Kind, pfKind protoreflect.Kind) bool {
	proto64 := pfKind == protoreflect.Uint64Kind || pfKind == protoreflect.Fixed64Kind
	switch goKind {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint:
		return true
	case reflect.Uint64:
		return proto64
	}
	return false
}

func unwrapPointer(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
