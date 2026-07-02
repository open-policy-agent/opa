#include "std.h"
#include "str.h"
#include "conversions.h"

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
    {
        opa_string_t *a = opa_cast_string(v);
        double n;
        // NOTE: we're only using opa_atof64 for validation here
        if (opa_atof64(a->v, a->len, &n) != 0)
        {
            return NULL;
        }

        return opa_number_ref(a->v, a->len);
    }
    default:
        return NULL;
    }
}
