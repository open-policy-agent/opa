#include "std.h"
#include "str.h"
#include "conversions.h"

opa_value *opa_to_number(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_NULL:
        return opa_number_int(0);
    case OPA_BOOLEAN:
    {
        opa_boolean_t *a = opa_cast_boolean(v);
        return a->v == FALSE ? opa_number_int(0) : opa_number_int(1);
    }
    case OPA_NUMBER:
       return v;
    case OPA_STRING:
    {
        opa_string_t *a = opa_cast_string(v);
        double n;

        if (opa_atof64(a->v, a->len, &n) != 0)
        {
            return NULL;
        }

        return opa_number_float(n);
    }
    default:
        return NULL;
    }
}