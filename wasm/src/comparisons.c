#include "value.h"
#include "comparisons.h"

OPA_BUILTIN
opa_value *opa_cmp_eq(opa_value *a, opa_value *b)
{
    return opa_boolean(opa_value_compare(a, b) == 0);
}

OPA_BUILTIN
opa_value *opa_cmp_neq(opa_value *a, opa_value *b)
{
    return opa_boolean(opa_value_compare(a, b) != 0);
}

OPA_BUILTIN
opa_value *opa_cmp_gt(opa_value *a, opa_value *b)
{
    return opa_boolean(opa_value_compare(a, b) > 0);
}

OPA_BUILTIN
opa_value *opa_cmp_gte(opa_value *a, opa_value *b)
{
    return opa_boolean(opa_value_compare(a, b) >= 0);
}

OPA_BUILTIN
opa_value *opa_cmp_lt(opa_value *a, opa_value *b)
{
    return opa_boolean(opa_value_compare(a, b) < 0);
}

OPA_BUILTIN
opa_value *opa_cmp_lte(opa_value *a, opa_value *b)
{
    return opa_boolean(opa_value_compare(a, b) <= 0);
}
