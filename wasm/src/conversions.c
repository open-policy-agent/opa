#include "std.h"
#include "str.h"
#include "conversions.h"
#include "value.h"

OPA_BUILTIN
opa_value *opa_to_number(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_NULL:
        return opa_number_int(0);
    case OPA_BOOLEAN:
    {
        opa_boolean_t *a = opa_cast_boolean(v);
        return opa_number_int(a->v ? 1 : 0);
    }
    case OPA_NUMBER:
       return v;
    case OPA_STRING:
        return opa_number_from_string(opa_cast_string(v));
    default:
        return NULL;
    }
}
