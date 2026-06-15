// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package protoroundtrip is a reflective Go ↔ protobuf codec used by the
// proto consistency tests to round-trip IR plans and bundle manifests
// through the wire format described in plan.proto and manifest.proto.
// Test-only — not part of OPA's runtime.
package protoroundtrip

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Codec round-trips Go values through proto wire format using a parsed
// proto FileDescriptor.
type Codec struct {
	file protoreflect.FileDescriptor

	// rootMsgs maps Go struct types to proto message names.
	rootMsgs map[reflect.Type]string
	// fieldOverrides[T]: Go-side JSON name → proto field name overrides.
	fieldOverrides map[reflect.Type]map[string]string
	// skipProtoFields[T]: proto fields ignored on encode AND decode (opaque).
	skipProtoFields map[reflect.Type]map[string]bool
	// skipGoFields[T]: Go fields (by Go field name) with no proto
	// counterpart; encoded as no-op, decoded as zero value.
	skipGoFields map[reflect.Type]map[string]bool
	// skipEmbedded[T]: embedded Go types NOT flattened into T (handled
	// elsewhere — typically promoted to an envelope).
	skipEmbedded map[reflect.Type]map[reflect.Type]bool
	// envelopes wraps a body type T in an envelope proto message.
	envelopes map[reflect.Type]EnvelopeSpec
	// scalarConverters[T]: Go type ↔ proto string conversions (e.g.
	// ast.Ref ↔ canonical dotted form, url.URL ↔ url string).
	scalarConverters map[reflect.Type]ScalarConverter
}

// ScalarConverter converts a Go value to/from a proto string field.
// Used when the Go field's type doesn't trivially map to a proto string
// (ast.Ref is `[]*Term`, url.URL is a struct) but the on-the-wire form
// is a string.
type ScalarConverter struct {
	// Encode converts the Go value (already pointer/interface deref'd by
	// the caller) to its string wire form.
	Encode func(reflect.Value) (string, error)
	// Decode parses a string wire value into a Go value of the registered
	// type. The returned reflect.Value is assigned via Set.
	Decode func(string) (reflect.Value, error)
}

// EnvelopeSpec wraps a body Go type in an envelope proto message during
// encode/decode. Used for ir.Stmt (Stmt envelope, body messages per kind,
// Location promoted to envelope) and ir.Val (Val envelope, scalar cases).
type EnvelopeSpec struct {
	Envelope string // envelope proto message name
	Oneof    string // oneof name on the envelope
	Case     string // proto oneof case name for this body type
	// PromotedEmbed names an embedded Go struct on the body whose fields
	// belong to the envelope rather than the body sub-message (e.g.
	// ir.Location on every Stmt body).
	PromotedEmbed reflect.Type
}

// NewCodec returns a Codec backed by file. Configure via the Register*
// and Set*/Skip* methods before calling Encode/Decode.
func NewCodec(file protoreflect.FileDescriptor) *Codec {
	return &Codec{
		file:             file,
		rootMsgs:         map[reflect.Type]string{},
		fieldOverrides:   map[reflect.Type]map[string]string{},
		skipProtoFields:  map[reflect.Type]map[string]bool{},
		skipGoFields:     map[reflect.Type]map[string]bool{},
		skipEmbedded:     map[reflect.Type]map[reflect.Type]bool{},
		envelopes:        map[reflect.Type]EnvelopeSpec{},
		scalarConverters: map[reflect.Type]ScalarConverter{},
	}
}

// RegisterRoot maps t to the proto message named msgName, so values of
// type t (or *t) can be passed directly to Encode/Decode.
func (c *Codec) RegisterRoot(t reflect.Type, msgName string) {
	c.rootMsgs[unwrapPointer(t)] = msgName
}

// SetFieldNameOverride sets the Go-JSON-name → proto-field-name map for t.
func (c *Codec) SetFieldNameOverride(t reflect.Type, overrides map[string]string) {
	c.fieldOverrides[unwrapPointer(t)] = overrides
}

// SkipProtoFields marks proto fields on the message for t as ignored
// (used for opaque fields like BuiltinFunc.decl).
func (c *Codec) SkipProtoFields(t reflect.Type, names ...string) {
	m := c.skipProtoFields[unwrapPointer(t)]
	if m == nil {
		m = map[string]bool{}
		c.skipProtoFields[unwrapPointer(t)] = m
	}
	for _, n := range names {
		m[n] = true
	}
}

// SkipGoFields names Go fields (by Go field name) on t that have no
// proto counterpart by design — encoded as no-op, decoded as zero
// value. Used for fields the proto schema deliberately omits, such as
// ir.BuiltinFunc.Decl (signatures live in the consumer's builtin
// registry, not the wire format).
func (c *Codec) SkipGoFields(t reflect.Type, names ...string) {
	m := c.skipGoFields[unwrapPointer(t)]
	if m == nil {
		m = map[string]bool{}
		c.skipGoFields[unwrapPointer(t)] = m
	}
	for _, n := range names {
		m[n] = true
	}
}

// SkipEmbedded opts out the listed embedded types from being flattened
// into t (they're handled elsewhere — typically on an envelope).
func (c *Codec) SkipEmbedded(t reflect.Type, embedded ...reflect.Type) {
	m := c.skipEmbedded[unwrapPointer(t)]
	if m == nil {
		m = map[reflect.Type]bool{}
		c.skipEmbedded[unwrapPointer(t)] = m
	}
	for _, e := range embedded {
		m[unwrapPointer(e)] = true
	}
}

// RegisterEnvelope wraps body type t in the envelope described by spec.
func (c *Codec) RegisterEnvelope(t reflect.Type, spec EnvelopeSpec) {
	c.envelopes[unwrapPointer(t)] = spec
}

// RegisterScalarConverter registers a converter for Go type t. When a
// proto string field's Go counterpart has type t, encode calls
// conv.Encode and decode calls conv.Decode.
func (c *Codec) RegisterScalarConverter(t reflect.Type, conv ScalarConverter) {
	c.scalarConverters[unwrapPointer(t)] = conv
}

// Encode serializes v (a value or pointer-to-value) as proto wire-format bytes.
func (c *Codec) Encode(v any) ([]byte, error) {
	rv := reflect.ValueOf(v)
	rv = derefValue(rv)
	t := rv.Type()
	msgName, ok := c.rootMsgs[t]
	if !ok {
		return nil, fmt.Errorf("protoroundtrip: no root message registered for Go type %s", t)
	}
	md, err := c.findMessage(msgName)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	if err := c.encodeStruct(rv, msg, nil); err != nil {
		return nil, err
	}
	return proto.Marshal(msg)
}

// Decode deserializes bytes into target (must be pointer-to-struct).
func (c *Codec) Decode(bytes []byte, target any) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("protoroundtrip: Decode target must be a non-nil pointer, got %s", rv.Kind())
	}
	rv = rv.Elem()
	t := rv.Type()
	msgName, ok := c.rootMsgs[t]
	if !ok {
		return fmt.Errorf("protoroundtrip: no root message registered for Go type %s", t)
	}
	md, err := c.findMessage(msgName)
	if err != nil {
		return err
	}
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(bytes, msg); err != nil {
		return fmt.Errorf("protoroundtrip: unmarshal: %w", err)
	}
	return c.decodeStruct(msg, rv, nil)
}

func (c *Codec) findMessage(name string) (protoreflect.MessageDescriptor, error) {
	md := lookupMessage(c.file, name)
	if md == nil {
		return nil, fmt.Errorf("protoroundtrip: proto message %q not found in %s", name, c.file.Path())
	}
	return md, nil
}

// lookupMessage searches file (and nested messages) for one named name.
func lookupMessage(file protoreflect.FileDescriptor, name string) protoreflect.MessageDescriptor {
	var find func(protoreflect.MessageDescriptors) protoreflect.MessageDescriptor
	find = func(msgs protoreflect.MessageDescriptors) protoreflect.MessageDescriptor {
		for i := range msgs.Len() {
			m := msgs.Get(i)
			if string(m.Name()) == name {
				return m
			}
			if got := find(m.Messages()); got != nil {
				return got
			}
		}
		return nil
	}
	return find(file.Messages())
}

// encodeStruct populates msg with the fields of rv (a struct value).
// envelopeFields, when non-nil, lists field names that should be set on
// an envelope rather than on the body — used while encoding a body type
// inside an envelope.
func (c *Codec) encodeStruct(rv reflect.Value, msg *dynamicpb.Message, envelopeFields *envelopeContext) error {
	t := rv.Type()
	skipFields := c.skipProtoFields[t]
	skipGo := c.skipGoFields[t]
	overrides := c.fieldOverrides[t]
	skipEmbeds := c.skipEmbedded[t]
	md := msg.Descriptor()

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if skipGo[f.Name] {
			continue
		}
		fv := rv.Field(i)

		if f.Anonymous {
			ft := unwrapPointer(f.Type)
			if ft.Kind() == reflect.Struct {
				if skipEmbeds[ft] {
					// Skipped here — handled elsewhere (e.g., promoted
					// to an envelope).
					continue
				}
				// Flatten: walk this embedded struct's fields onto the
				// same proto message.
				if err := c.encodeStruct(derefValue(fv), msg, envelopeFields); err != nil {
					return err
				}
				continue
			}
		}

		jsonName, ok := jsonFieldName(f)
		if !ok {
			continue
		}
		protoName := jsonName
		if remap, has := overrides[jsonName]; has {
			protoName = remap
		}

		if skipFields[protoName] {
			continue
		}

		// Some fields belong to a parent envelope rather than this
		// message — skip them here; they're set by the envelope path.
		if envelopeFields != nil && envelopeFields.belongsToEnvelope(protoName) {
			continue
		}

		fd := md.Fields().ByName(protoreflect.Name(protoName))
		if fd == nil {
			return fmt.Errorf("protoroundtrip: %s: Go field %s maps to proto field %q but message has no such field", t.Name(), f.Name, protoName)
		}

		if err := c.encodeField(fv, fd, msg); err != nil {
			return fmt.Errorf("%s.%s: %w", t.Name(), f.Name, err)
		}
	}
	return nil
}

// encodeField sets a single field on msg from rv.
func (c *Codec) encodeField(rv reflect.Value, fd protoreflect.FieldDescriptor, msg *dynamicpb.Message) error {
	// Nil pointer / nil interface = proto "absent". Skip.
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil
	}

	if fd.IsList() {
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return fmt.Errorf("expected slice for repeated proto field, got %s", rv.Kind())
		}
		if rv.Len() == 0 {
			return nil
		}
		list := msg.Mutable(fd).List()
		for i := range rv.Len() {
			elem, err := c.encodeScalarOrMessage(derefValue(rv.Index(i)), fd, list.NewElement())
			if err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
			list.Append(elem)
		}
		return nil
	}

	if fd.IsMap() {
		if rv.Kind() != reflect.Map {
			return fmt.Errorf("expected map for proto map field, got %s", rv.Kind())
		}
		if rv.Len() == 0 {
			return nil
		}
		mapField := msg.Mutable(fd).Map()
		valFD := fd.MapValue()
		iter := rv.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			val, err := c.encodeScalarOrMessage(derefValue(iter.Value()), valFD, mapField.NewValue())
			if err != nil {
				return fmt.Errorf("[%q]: %w", key, err)
			}
			mapField.Set(protoreflect.ValueOfString(key).MapKey(), val)
		}
		return nil
	}

	val, err := c.encodeScalarOrMessage(rv, fd, msg.NewField(fd))
	if err != nil {
		return err
	}
	msg.Set(fd, val)
	return nil
}

// encodeScalarOrMessage converts rv to a protoreflect.Value compatible
// with fd. For message-typed fields, scratch is a fresh message of the
// right type (created by the caller via NewElement / NewValue / NewField).
func (c *Codec) encodeScalarOrMessage(rv reflect.Value, fd protoreflect.FieldDescriptor, scratch protoreflect.Value) (protoreflect.Value, error) {
	rv = derefValue(rv)
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(rv.Bool()), nil
	case protoreflect.StringKind:
		if conv, ok := c.scalarConverters[rv.Type()]; ok {
			s, err := conv.Encode(rv)
			if err != nil {
				return protoreflect.Value{}, fmt.Errorf("scalar converter for %s: %w", rv.Type(), err)
			}
			return protoreflect.ValueOfString(s), nil
		}
		return protoreflect.ValueOfString(rv.String()), nil
	case protoreflect.BytesKind:
		bs := []byte{}
		if rv.IsValid() && !rv.IsZero() {
			bs = rv.Bytes()
		}
		return protoreflect.ValueOfBytes(bs), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(rv.Int())), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(rv.Int()), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(uint32(rv.Uint())), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(rv.Uint()), nil
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(rv.Float())), nil
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(rv.Float()), nil
	case protoreflect.MessageKind:
		// Special-case google.protobuf.Struct for free-form Go map[string]any
		// or empty interface fields.
		if string(fd.Message().FullName()) == "google.protobuf.Struct" {
			return c.encodeStructpb(rv, scratch)
		}
		// Special-case google.protobuf.Value for free-form Go any /
		// map[string]any / scalar fields where the top-level may be a
		// non-object JSON value.
		if string(fd.Message().FullName()) == "google.protobuf.Value" {
			return c.encodeValuepb(rv, scratch)
		}
		// General message-typed field: rv is a Go struct (or nil).
		if !rv.IsValid() {
			return scratch, nil
		}
		// Body Go type? Wrap in envelope.
		if env, ok := c.envelopes[rv.Type()]; ok {
			env := env
			return c.encodeEnvelope(rv, env, scratch)
		}
		// Plain nested message.
		sub := scratch.Message().(*dynamicpb.Message)
		if err := c.encodeStruct(rv, sub, nil); err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfMessage(sub), nil
	}
	return protoreflect.Value{}, fmt.Errorf("unsupported proto Kind=%s", fd.Kind())
}

// encodeEnvelope wraps a body value in its envelope message: populates
// the envelope's promoted-embed fields from the body's embedded struct,
// then populates the body sub-message and sets the appropriate oneof case.
// Scalar oneof cases (where the case field's Kind is not MessageKind) are
// handled by treating rv directly as the scalar value.
func (c *Codec) encodeEnvelope(rv reflect.Value, env EnvelopeSpec, scratch protoreflect.Value) (protoreflect.Value, error) {
	envMD, err := c.findMessage(env.Envelope)
	if err != nil {
		return protoreflect.Value{}, err
	}
	envMsg := dynamicpb.NewMessage(envMD)

	// Populate the envelope's promoted-from-embed fields, if any.
	envFieldNames := map[string]bool{}
	if env.PromotedEmbed != nil {
		emb := findEmbedded(rv, env.PromotedEmbed)
		if emb.IsValid() {
			if err := c.encodeStruct(emb, envMsg, nil); err != nil {
				return protoreflect.Value{}, fmt.Errorf("envelope %s: promoted embed %s: %w", env.Envelope, env.PromotedEmbed.Name(), err)
			}
			for i := range env.PromotedEmbed.NumField() {
				ef := env.PromotedEmbed.Field(i)
				if !ef.IsExported() || ef.Anonymous {
					continue
				}
				name, ok := jsonFieldName(ef)
				if ok {
					envFieldNames[name] = true
				}
			}
		}
	}

	// Find the oneof case descriptor.
	oo := envMD.Oneofs().ByName(protoreflect.Name(env.Oneof))
	if oo == nil {
		return protoreflect.Value{}, fmt.Errorf("envelope %s has no oneof %q", env.Envelope, env.Oneof)
	}
	caseFD := oo.Fields().ByName(protoreflect.Name(env.Case))
	if caseFD == nil {
		return protoreflect.Value{}, fmt.Errorf("envelope %s: oneof %s has no case %q", env.Envelope, env.Oneof, env.Case)
	}

	if caseFD.Kind() == protoreflect.MessageKind {
		bodyMsg := dynamicpb.NewMessage(caseFD.Message())
		if err := c.encodeStruct(rv, bodyMsg, &envelopeContext{fields: envFieldNames}); err != nil {
			return protoreflect.Value{}, fmt.Errorf("envelope %s case %s: %w", env.Envelope, env.Case, err)
		}
		envMsg.Set(caseFD, protoreflect.ValueOfMessage(bodyMsg))
	} else {
		// Scalar case (Val.bool, Val.local, Val.string_index): the
		// body value IS the scalar.
		val, err := c.encodeScalarValue(rv, caseFD)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("envelope %s case %s: %w", env.Envelope, env.Case, err)
		}
		envMsg.Set(caseFD, val)
	}
	return protoreflect.ValueOfMessage(envMsg), nil
}

// encodeScalarValue converts a scalar Go value to a protoreflect.Value of
// the kind required by fd. Used by envelope scalar cases.
func (*Codec) encodeScalarValue(rv reflect.Value, fd protoreflect.FieldDescriptor) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(rv.Bool()), nil
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(rv.String()), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(rv.Int())), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(rv.Int()), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(uint32(rv.Uint())), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(rv.Uint()), nil
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(rv.Float())), nil
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(rv.Float()), nil
	}
	return protoreflect.Value{}, fmt.Errorf("unsupported scalar Kind=%s", fd.Kind())
}

// encodeStructpb converts a Go map[string]any (or any-typed value) to a
// google.protobuf.Struct.
func (*Codec) encodeStructpb(rv reflect.Value, scratch protoreflect.Value) (protoreflect.Value, error) {
	if !rv.IsValid() || (rv.Kind() == reflect.Map && rv.IsNil()) {
		empty, err := structpb.NewStruct(nil)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("structpb: %w", err)
		}
		return protoreflect.ValueOfMessage(empty.ProtoReflect()), nil
	}
	var raw map[string]any
	switch rv.Kind() {
	case reflect.Map:
		raw = make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			raw[iter.Key().String()] = iter.Value().Interface()
		}
	case reflect.Interface, reflect.Struct, reflect.Slice, reflect.Pointer:
		// Fallback: try to coerce via Interface().
		if v, ok := rv.Interface().(map[string]any); ok {
			raw = v
		}
	}
	s, err := structpb.NewStruct(raw)
	if err != nil {
		return protoreflect.Value{}, fmt.Errorf("structpb: %w", err)
	}
	return protoreflect.ValueOfMessage(s.ProtoReflect()), nil
}

// encodeValuepb converts an arbitrary Go value (typically the underlying
// of an `any` field) to a google.protobuf.Value. Used for SchemaAnnotation.Definition
// where the wire shape is a single JSON value (scalar, list, or object),
// not a map.
func (*Codec) encodeValuepb(rv reflect.Value, _ protoreflect.Value) (protoreflect.Value, error) {
	if !rv.IsValid() || (rv.Kind() == reflect.Interface && rv.IsNil()) {
		nullVal := structpb.NewNullValue()
		return protoreflect.ValueOfMessage(nullVal.ProtoReflect()), nil
	}
	val, err := structpb.NewValue(rv.Interface())
	if err != nil {
		return protoreflect.Value{}, fmt.Errorf("structpb value: %w", err)
	}
	return protoreflect.ValueOfMessage(val.ProtoReflect()), nil
}

// envelopeContext tracks which field names live on a parent envelope
// (and thus should be skipped when encoding the body sub-message).
type envelopeContext struct {
	fields map[string]bool
}

func (e *envelopeContext) belongsToEnvelope(name string) bool {
	return e != nil && e.fields[name]
}

// reverseEnvelope looks up the Go body type for a given (envelope, case)
// pair — needed during decoding when the codec sees an envelope and has
// to materialize the appropriate Go body.
func (c *Codec) reverseEnvelope(envelope, caseName string) (reflect.Type, EnvelopeSpec, bool) {
	for bodyType, spec := range c.envelopes {
		if spec.Envelope == envelope && spec.Case == caseName {
			return bodyType, spec, true
		}
	}
	return nil, EnvelopeSpec{}, false
}

// envelopesForMessage returns every EnvelopeSpec that targets the named
// envelope message. Used by the decoder to know which oneof cases to
// dispatch on for a given envelope.
func (c *Codec) envelopesForMessage(envelope string) []EnvelopeSpec {
	var out []EnvelopeSpec
	for _, spec := range c.envelopes {
		if spec.Envelope == envelope {
			out = append(out, spec)
		}
	}
	return out
}

// decodeStruct populates rv (a struct value) from msg.
func (c *Codec) decodeStruct(msg protoreflect.Message, rv reflect.Value, envelopeFields *envelopeContext) error {
	t := rv.Type()
	overrides := c.fieldOverrides[t]
	skipFields := c.skipProtoFields[t]
	skipGo := c.skipGoFields[t]
	skipEmbeds := c.skipEmbedded[t]
	md := msg.Descriptor()

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if skipGo[f.Name] {
			continue
		}
		fv := rv.Field(i)

		if f.Anonymous {
			ft := unwrapPointer(f.Type)
			if ft.Kind() == reflect.Struct {
				if skipEmbeds[ft] {
					continue
				}
				// Recurse into the embedded struct, populating it from
				// the same proto message.
				target := fv
				if target.Kind() == reflect.Pointer {
					if target.IsNil() {
						target.Set(reflect.New(target.Type().Elem()))
					}
					target = target.Elem()
				}
				if err := c.decodeStruct(msg, target, envelopeFields); err != nil {
					return err
				}
				continue
			}
		}

		jsonName, ok := jsonFieldName(f)
		if !ok {
			continue
		}
		protoName := jsonName
		if remap, has := overrides[jsonName]; has {
			protoName = remap
		}

		if skipFields[protoName] {
			continue
		}

		// Field belongs to the envelope (set by the envelope walker)?
		// Skip it here.
		if envelopeFields != nil && envelopeFields.belongsToEnvelope(protoName) {
			continue
		}

		fd := md.Fields().ByName(protoreflect.Name(protoName))
		if fd == nil {
			return fmt.Errorf("protoroundtrip: %s: Go field %s maps to proto field %q but message has no such field", t.Name(), f.Name, protoName)
		}
		if !msg.Has(fd) && !fd.IsList() && !fd.IsMap() {
			// Field unset on the wire; leave the Go field zero.
			continue
		}
		if err := c.decodeField(msg.Get(fd), fd, fv); err != nil {
			return fmt.Errorf("%s.%s: %w", t.Name(), f.Name, err)
		}
	}
	return nil
}

// decodeField unpacks a single proto field value into the Go reflect.Value.
func (c *Codec) decodeField(v protoreflect.Value, fd protoreflect.FieldDescriptor, rv reflect.Value) error {
	// Allocate through any pointer indirection (e.g. *[]string, *int).
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}
	if fd.IsList() {
		return c.decodeList(v.List(), fd, rv)
	}
	if fd.IsMap() {
		return c.decodeMap(v.Map(), fd, rv)
	}
	return c.decodeScalarOrMessage(v, fd, rv)
}

func (c *Codec) decodeList(list protoreflect.List, fd protoreflect.FieldDescriptor, rv reflect.Value) error {
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return fmt.Errorf("expected slice for repeated proto field, got %s", rv.Kind())
	}
	n := list.Len()
	out := reflect.MakeSlice(rv.Type(), n, n)
	for i := range n {
		if err := c.decodeScalarOrMessage(list.Get(i), fd, out.Index(i)); err != nil {
			return fmt.Errorf("[%d]: %w", i, err)
		}
	}
	rv.Set(out)
	return nil
}

func (c *Codec) decodeMap(m protoreflect.Map, fd protoreflect.FieldDescriptor, rv reflect.Value) error {
	if rv.Kind() != reflect.Map {
		return fmt.Errorf("expected map for proto map field, got %s", rv.Kind())
	}
	mt := rv.Type()
	out := reflect.MakeMapWithSize(mt, m.Len())
	valFD := fd.MapValue()
	var rangeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		valRV := reflect.New(mt.Elem()).Elem()
		if err := c.decodeScalarOrMessage(v, valFD, valRV); err != nil {
			rangeErr = fmt.Errorf("[%q]: %w", k.String(), err)
			return false
		}
		out.SetMapIndex(reflect.ValueOf(k.String()).Convert(mt.Key()), valRV)
		return true
	})
	if rangeErr != nil {
		return rangeErr
	}
	rv.Set(out)
	return nil
}

func (c *Codec) decodeScalarOrMessage(v protoreflect.Value, fd protoreflect.FieldDescriptor, rv reflect.Value) error {
	// Handle pointer Go targets — allocate as needed.
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	switch fd.Kind() {
	case protoreflect.BoolKind:
		rv.SetBool(v.Bool())
	case protoreflect.StringKind:
		if conv, ok := c.scalarConverters[rv.Type()]; ok {
			decoded, err := conv.Decode(v.String())
			if err != nil {
				return fmt.Errorf("scalar converter for %s: %w", rv.Type(), err)
			}
			if decoded.Type() != rv.Type() {
				if !decoded.Type().ConvertibleTo(rv.Type()) {
					return fmt.Errorf("scalar converter for %s returned %s; not convertible", rv.Type(), decoded.Type())
				}
				decoded = decoded.Convert(rv.Type())
			}
			rv.Set(decoded)
		} else {
			rv.SetString(v.String())
		}
	case protoreflect.BytesKind:
		rv.SetBytes(v.Bytes())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		rv.SetInt(v.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		rv.SetUint(v.Uint())
	case protoreflect.FloatKind:
		rv.SetFloat(v.Float())
	case protoreflect.DoubleKind:
		rv.SetFloat(v.Float())
	case protoreflect.MessageKind:
		return c.decodeMessage(v.Message(), fd, rv)
	default:
		return fmt.Errorf("unsupported proto Kind=%s", fd.Kind())
	}
	return nil
}

// decodeMessage populates rv from a sub-message. Handles the
// google.protobuf.Struct special case, envelope dispatch (oneof), and
// plain nested messages.
func (c *Codec) decodeMessage(sub protoreflect.Message, fd protoreflect.FieldDescriptor, rv reflect.Value) error {
	if string(fd.Message().FullName()) == "google.protobuf.Struct" {
		return c.decodeStructpb(sub, rv)
	}
	if string(fd.Message().FullName()) == "google.protobuf.Value" {
		return c.decodeValuepb(sub, rv)
	}

	// Is this an envelope? Check by message name.
	envName := string(fd.Message().Name())
	if specs := c.envelopesForMessage(envName); len(specs) > 0 {
		return c.decodeEnvelope(sub, specs, rv)
	}

	// Plain nested message — rv must be a struct (or interface holding one).
	target := rv
	if rv.Kind() == reflect.Interface {
		// Caller is responsible for setting up the concrete type via
		// envelope dispatch. A plain interface field with a non-envelope
		// message type isn't supported.
		return errors.New("plain message-typed proto field maps to Go interface — register an envelope or use a concrete struct type")
	}
	if target.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct for message-typed proto field, got %s", target.Kind())
	}
	return c.decodeStruct(sub, target, nil)
}

// decodeEnvelope inspects the oneof on sub, picks the matching Go body
// type, and populates rv (an interface or struct field).
func (c *Codec) decodeEnvelope(sub protoreflect.Message, specs []EnvelopeSpec, rv reflect.Value) error {
	envMD := sub.Descriptor()

	// Find the oneof case that's set.
	if len(specs) == 0 {
		return errors.New("no envelope specs registered")
	}
	oneofName := specs[0].Oneof
	oo := envMD.Oneofs().ByName(protoreflect.Name(oneofName))
	if oo == nil {
		return fmt.Errorf("envelope %s has no oneof %q", envMD.Name(), oneofName)
	}
	whichFD := sub.WhichOneof(oo)
	if whichFD == nil {
		return fmt.Errorf("envelope %s: no oneof case set", envMD.Name())
	}
	caseName := string(whichFD.Name())

	bodyType, spec, ok := c.reverseEnvelope(string(envMD.Name()), caseName)
	if !ok {
		return fmt.Errorf("envelope %s: unrecognized oneof case %q", envMD.Name(), caseName)
	}

	// Allocate a new body value.
	bodyPtr := reflect.New(bodyType)
	bodyVal := bodyPtr.Elem()

	// Populate the promoted-embed fields from the envelope.
	if spec.PromotedEmbed != nil {
		emb := findEmbedded(bodyVal, spec.PromotedEmbed)
		if emb.IsValid() && emb.CanAddr() {
			if err := c.decodeStruct(sub, emb, nil); err != nil {
				return fmt.Errorf("envelope %s promoted embed: %w", envMD.Name(), err)
			}
		}
	}

	// Track which fields were set by the embed so the body decoder
	// doesn't try to read them from the body sub-message.
	envFieldNames := map[string]bool{}
	if spec.PromotedEmbed != nil {
		for i := range spec.PromotedEmbed.NumField() {
			ef := spec.PromotedEmbed.Field(i)
			if !ef.IsExported() || ef.Anonymous {
				continue
			}
			if name, ok := jsonFieldName(ef); ok {
				envFieldNames[name] = true
			}
		}
	}

	// Decode the body — message case populates a sub-struct; scalar
	// case populates the body value directly.
	if whichFD.Kind() == protoreflect.MessageKind {
		caseSub := sub.Get(whichFD).Message()
		if err := c.decodeStruct(caseSub, bodyVal, &envelopeContext{fields: envFieldNames}); err != nil {
			return fmt.Errorf("envelope %s case %s body: %w", envMD.Name(), caseName, err)
		}
	} else {
		caseVal := sub.Get(whichFD)
		if err := decodeScalarValue(caseVal, whichFD, bodyVal); err != nil {
			return fmt.Errorf("envelope %s case %s body: %w", envMD.Name(), caseName, err)
		}
	}

	// Assign into rv. If rv is an interface, prefer the value form when
	// it satisfies the interface (matches how OPA's planner stores
	// scalar Val values directly, not as pointers); otherwise fall
	// back to the pointer form (Stmt bodies all use pointer receivers).
	switch rv.Kind() {
	case reflect.Interface:
		ifaceT := rv.Type()
		switch {
		case bodyVal.Type().Implements(ifaceT):
			rv.Set(bodyVal)
		case bodyPtr.Type().Implements(ifaceT):
			rv.Set(bodyPtr)
		default:
			return fmt.Errorf("envelope %s case %s: neither %s nor *%s implements %s",
				envMD.Name(), caseName, bodyType, bodyType, ifaceT)
		}
	case reflect.Pointer:
		rv.Set(bodyPtr)
	case reflect.Struct:
		rv.Set(bodyVal)
	default:
		return fmt.Errorf("cannot assign envelope body to Go kind %s", rv.Kind())
	}
	return nil
}

// decodeScalarValue copies a scalar protoreflect.Value into the Go
// reflect.Value rv (already pointer-deref'd by the caller).
func decodeScalarValue(v protoreflect.Value, fd protoreflect.FieldDescriptor, rv reflect.Value) error {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		rv.SetBool(v.Bool())
	case protoreflect.StringKind:
		rv.SetString(v.String())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		rv.SetInt(v.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		rv.SetUint(v.Uint())
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		rv.SetFloat(v.Float())
	default:
		return fmt.Errorf("unsupported scalar Kind=%s", fd.Kind())
	}
	return nil
}

// decodeStructpb converts a google.protobuf.Struct back to a Go map[string]any.
func (*Codec) decodeStructpb(sub protoreflect.Message, rv reflect.Value) error {
	// Marshal sub to a *structpb.Struct so we can use AsMap().
	s := &structpb.Struct{}
	bs, err := proto.Marshal(sub.Interface())
	if err != nil {
		return fmt.Errorf("structpb marshal: %w", err)
	}
	if err := proto.Unmarshal(bs, s); err != nil {
		return fmt.Errorf("structpb unmarshal: %w", err)
	}
	asMap := s.AsMap()

	// rv may be map[string]any (typed) or interface{}.
	if rv.Kind() == reflect.Interface {
		rv.Set(reflect.ValueOf(asMap))
		return nil
	}
	if rv.Kind() != reflect.Map {
		return fmt.Errorf("expected map for google.protobuf.Struct, got %s", rv.Kind())
	}
	if len(asMap) == 0 {
		// Match the zero-value semantics of an empty Go map (nil).
		return nil
	}
	out := reflect.MakeMapWithSize(rv.Type(), len(asMap))
	for k, v := range asMap {
		out.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
	}
	rv.Set(out)
	return nil
}

// decodeValuepb converts a google.protobuf.Value back to a Go any.
// Mirrors decodeStructpb but for the single-value variant.
func (*Codec) decodeValuepb(sub protoreflect.Message, rv reflect.Value) error {
	val := &structpb.Value{}
	bs, err := proto.Marshal(sub.Interface())
	if err != nil {
		return fmt.Errorf("structpb value marshal: %w", err)
	}
	if err := proto.Unmarshal(bs, val); err != nil {
		return fmt.Errorf("structpb value unmarshal: %w", err)
	}
	asInterface := val.AsInterface()
	if rv.Kind() != reflect.Interface {
		return fmt.Errorf("expected interface for google.protobuf.Value, got %s", rv.Kind())
	}
	if asInterface == nil {
		return nil
	}
	rv.Set(reflect.ValueOf(asInterface))
	return nil
}

// findEmbedded returns the reflect.Value of an embedded struct of type
// embT inside rv (a struct value). Returns the zero Value if not found.
func findEmbedded(rv reflect.Value, embT reflect.Type) reflect.Value {
	rv = derefValue(rv)
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	t := rv.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.Anonymous {
			continue
		}
		ft := unwrapPointer(f.Type)
		if ft == embT {
			return derefValue(rv.Field(i))
		}
		if ft.Kind() == reflect.Struct {
			if got := findEmbedded(rv.Field(i), embT); got.IsValid() {
				return got
			}
		}
	}
	return reflect.Value{}
}

// jsonFieldName mirrors the helper in protoschemacheck.
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

func unwrapPointer(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func derefValue(v reflect.Value) reflect.Value {
	for {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if v.IsNil() {
				return v
			}
			v = v.Elem()
		default:
			return v
		}
	}
}
