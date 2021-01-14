#include "std.h"
#include "types.h"

OPA_BUILTIN
opa_value *opa_types_is_number(opa_value *v)
{
    return opa_value_type(v) == OPA_NUMBER ? opa_boolean(true) : NULL;
}

OPA_BUILTIN
opa_value *opa_types_is_string(opa_value *v)
{
    return opa_value_type(v) == OPA_STRING ? opa_boolean(true) : NULL;
}

OPA_BUILTIN
opa_value *opa_types_is_boolean(opa_value *v)
{
    return opa_value_type(v) == OPA_BOOLEAN ? opa_boolean(true) : NULL;
}

OPA_BUILTIN
opa_value *opa_types_is_array(opa_value *v)
{
    return opa_value_type(v) == OPA_ARRAY ? opa_boolean(true) : NULL;
}

OPA_BUILTIN
opa_value *opa_types_is_set(opa_value *v)
{
    return opa_value_type(v) == OPA_SET ? opa_boolean(true) : NULL;
}

OPA_BUILTIN
opa_value *opa_types_is_object(opa_value *v)
{
    return opa_value_type(v) == OPA_OBJECT ? opa_boolean(true) : NULL;
}

OPA_BUILTIN
opa_value *opa_types_is_null(opa_value *v)
{
    return opa_value_type(v) == OPA_NULL ? opa_boolean(true) : NULL;
}

OPA_BUILTIN
opa_value *opa_types_name(opa_value *v)
{
    switch (v->type)
    {
    case OPA_NULL:
        return opa_string("null", 4);
    case OPA_BOOLEAN:
        return opa_string("boolean", 7);
    case OPA_NUMBER:
        return opa_string("number", 6);
    case OPA_STRING:
        return opa_string("string", 6);
    case OPA_ARRAY:
        return opa_string("array", 5);
    case OPA_OBJECT:
        return opa_string("object", 6);
    case OPA_SET:
        return opa_string("set", 3);
    default:
        return NULL;
    }
}
