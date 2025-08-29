package sqlbuilder

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

type structFields struct {
	noTag  *structTaggedFields
	tagged map[string]*structTaggedFields
}

type structTaggedFields struct {
	// All columns for SELECT.
	ForRead     []*structField
	colsForRead map[string]*structField

	// All columns which can be used in INSERT and UPDATE.
	ForWrite     []*structField
	colsForWrite map[string]struct{}
}

type structField struct {
	Name     string
	Alias    string
	As       string
	Tags     []string
	IsQuoted bool
	DBTag    string
	Field    reflect.StructField

	omitEmptyTags omitEmptyTagMap
}

type structFieldsParser func() *structFields

func makeDefaultFieldsParser(t reflect.Type) structFieldsParser {
	return makeFieldsParser(t, nil, true)
}

func makeCustomFieldsParser(t reflect.Type, mapper FieldMapperFunc) structFieldsParser {
	return makeFieldsParser(t, mapper, false)
}

func makeFieldsParser(t reflect.Type, mapper FieldMapperFunc, useDefault bool) structFieldsParser {
	var once sync.Once
	sfs := &structFields{
		noTag:  makeStructTaggedFields(),
		tagged: map[string]*structTaggedFields{},
	}

	return func() *structFields {
		once.Do(func() {
			if useDefault {
				mapper = DefaultFieldMapper
			}

			sfs.parse(t, mapper, "")
		})

		return sfs
	}
}

func (sfs *structFields) parse(t reflect.Type, mapper FieldMapperFunc, prefix string) {
	l := t.NumField()
	var anonymous []reflect.StructField

	for i := 0; i < l; i++ {
		field := t.Field(i)

		// Skip unexported fields that are not embedded structs.
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}

		if field.Anonymous {
			ft := field.Type

			// If field is an anonymous struct or pointer to struct, parse it later.
			if k := ft.Kind(); k == reflect.Struct || (k == reflect.Ptr && ft.Elem().Kind() == reflect.Struct) {
				anonymous = append(anonymous, field)
				continue
			}
		}

		// Parse DBTag.
		alias, dbtag := DefaultGetAlias(&field)

		if alias == "-" {
			continue
		}

		if alias == "" {
			alias = field.Name
			if mapper != nil {
				alias = mapper(alias)
			}
		}

		// Parse FieldOpt.
		fieldopt := field.Tag.Get(FieldOpt)
		opts := optRegex.FindAllString(fieldopt, -1)
		isQuoted := false
		omitEmptyTags := omitEmptyTagMap{}

		for _, opt := range opts {
			optMap := getOptMatchedMap(opt)

			switch optMap[optName] {
			case fieldOptOmitEmpty:
				tags := getTagsFromOptParams(optMap[optParams])

				for _, tag := range tags {
					omitEmptyTags[tag] = struct{}{}
				}

			case fieldOptWithQuote:
				isQuoted = true
			}
		}

		// Parse FieldAs.
		fieldas := field.Tag.Get(FieldAs)

		// Parse FieldTag.
		fieldtag := field.Tag.Get(FieldTag)
		tags := splitTags(fieldtag)

		// Make struct field.
		structField := &structField{
			Name:          field.Name,
			Alias:         alias,
			As:            fieldas,
			Tags:          tags,
			IsQuoted:      isQuoted,
			DBTag:         dbtag,
			Field:         field,
			omitEmptyTags: omitEmptyTags,
		}

		// Make sure all fields can be added to noTag without conflict.
		sfs.noTag.Add(structField)

		for _, tag := range tags {
			sfs.taggedFields(tag).Add(structField)
		}
	}

	for _, field := range anonymous {
		ft := dereferencedType(field.Type)
		sfs.parse(ft, mapper, prefix+field.Name+".")
	}
}

func (sfs *structFields) FilterTags(with, without []string) *structTaggedFields {
	if len(with) == 0 && len(without) == 0 {
		return sfs.noTag
	}

	// Simply return the tagged fields.
	if len(with) == 1 && len(without) == 0 {
		return sfs.tagged[with[0]]
	}

	// Find out all with and without fields.
	taggedFields := makeStructTaggedFields()
	filteredReadFields := make(map[string]struct{}, len(sfs.noTag.colsForRead))

	for _, tag := range without {
		if field, ok := sfs.tagged[tag]; ok {
			for k := range field.colsForRead {
				filteredReadFields[k] = struct{}{}
			}
		}
	}

	if len(with) == 0 {
		for _, field := range sfs.noTag.ForRead {
			k := field.Key()

			if _, ok := filteredReadFields[k]; !ok {
				taggedFields.Add(field)
			}
		}
	} else {
		for _, tag := range with {
			if fields, ok := sfs.tagged[tag]; ok {
				for _, field := range fields.ForRead {
					k := field.Key()

					if _, ok := filteredReadFields[k]; !ok {
						taggedFields.Add(field)
					}
				}
			}
		}
	}

	return taggedFields
}

func (sfs *structFields) taggedFields(tag string) *structTaggedFields {
	fields, ok := sfs.tagged[tag]

	if !ok {
		fields = makeStructTaggedFields()
		sfs.tagged[tag] = fields
	}

	return fields
}

func makeStructTaggedFields() *structTaggedFields {
	return &structTaggedFields{
		colsForRead:  map[string]*structField{},
		colsForWrite: map[string]struct{}{},
	}
}

// Add a new field to stfs.
// If field's key exists in stfs.fields, the field is ignored.
func (stfs *structTaggedFields) Add(field *structField) {
	key := field.Key()

	if _, ok := stfs.colsForRead[key]; !ok {
		stfs.colsForRead[key] = field
		stfs.ForRead = append(stfs.ForRead, field)
	}

	key = field.Alias

	if _, ok := stfs.colsForWrite[key]; !ok {
		stfs.colsForWrite[key] = struct{}{}
		stfs.ForWrite = append(stfs.ForWrite, field)
	}
}

// Cols returns the fields whose key is one of cols.
// If any column in cols doesn't exist in sfs.fields, returns nil.
func (stfs *structTaggedFields) Cols(cols []string) []*structField {
	fields := make([]*structField, 0, len(cols))

	for _, col := range cols {
		field := stfs.colsForRead[col]

		if field == nil {
			return nil
		}

		fields = append(fields, field)
	}

	return fields
}

// Key returns the key name to identify a field.
func (sf *structField) Key() string {
	if sf.As != "" {
		return sf.As
	}

	if sf.Alias != "" {
		return sf.Alias
	}

	return sf.Name
}

// NameForSelect returns the name for SELECT.
func (sf *structField) NameForSelect(flavor Flavor) string {
	if sf.As == "" {
		return sf.Quote(flavor)
	}

	return fmt.Sprintf("%s AS %s", sf.Quote(flavor), sf.As)
}

// Quote the Alias in sf with flavor.
func (sf *structField) Quote(flavor Flavor) string {
	if !sf.IsQuoted {
		return sf.Alias
	}

	return flavor.Quote(sf.Alias)
}

// ShouldOmitEmpty returns true only if any one of tags is in the omitted tags map.
func (sf *structField) ShouldOmitEmpty(tags ...string) (ret bool) {
	omit := sf.omitEmptyTags

	if len(omit) == 0 {
		return
	}

	// Always check default tag.
	if _, ret = omit[""]; ret {
		return
	}

	for _, tag := range tags {
		if _, ret = omit[tag]; ret {
			return
		}
	}

	return
}

type omitEmptyTagMap map[string]struct{}

func getOptMatchedMap(opt string) (res map[string]string) {
	res = map[string]string{}
	sm := optRegex.FindStringSubmatch(opt)

	for i, name := range optRegex.SubexpNames() {
		if name != "" {
			res[name] = sm[i]
		}
	}

	return
}

func getTagsFromOptParams(opts string) (tags []string) {
	tags = splitTags(opts)

	if len(tags) == 0 {
		tags = append(tags, "")
	}

	return
}

func splitTags(fieldtag string) (tags []string) {
	parts := strings.Split(fieldtag, ",")

	for _, v := range parts {
		tag := strings.TrimSpace(v)

		if tag == "" {
			continue
		}

		tags = append(tags, tag)
	}

	return
}
