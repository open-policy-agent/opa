# METADATA
# description: |
#   Converts the response of Solr's Schema API (GET /solr/<core>/schema) into
#   the `schemas` annotation of a filter policy whose unknown is `input.doc`:
#
#     curl -s http://localhost:8983/solr/<core>/schema > solr.json
#     opa eval -f raw -d solr_schema.rego -i solr.json data.solr.annotation
#
#   A multiValued field is an array, any other a single value. Dynamic fields,
#   and fields of a type without a JSON counterpart, are left out.
package solr

annotation := concat("\n", array.concat(["# schemas:"], [$"#   - input.doc{_path(name)}: {json.marshal(schema)}" |
	some name, schema in schemas
]))

schemas[field.name] := _schema(multi_valued, json_type) if {
	some field in input.schema.fields
	field_type := field_types[field.type]
	json_type := json_types[field_type.class]
	multi_valued := object.get(field, "multiValued", object.get(field_type, "multiValued", false))
}

field_types[t.name] := t if some t in input.schema.fieldTypes

json_types := {
	"solr.StrField": "string",
	"solr.TextField": "string",
	"solr.SortableTextField": "string",
	"solr.UUIDField": "string",
	"solr.EnumFieldType": "string",
	"solr.DatePointField": "string",
	"solr.DateRangeField": "string",
	"solr.IntPointField": "integer",
	"solr.LongPointField": "integer",
	"solr.FloatPointField": "number",
	"solr.DoublePointField": "number",
	"solr.BoolField": "boolean",
}

_schema(true, json_type) := {"type": "array", "items": {"type": json_type}}

_schema(false, json_type) := {"type": json_type}

# `origin.country` is the reference input.doc.origin.country.
_path(name) := concat("", [_segment(s) | some s in split(name, ".")])

_segment(s) := $".{s}" if regex.match(`^[a-zA-Z_][a-zA-Z0-9_]*$`, s)

else := $"[{json.marshal(s)}]"
