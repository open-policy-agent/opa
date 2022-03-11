#include "std.h"
#include "value.h"

OPA_BUILTIN
opa_value *opa_array_concat(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_ARRAY || opa_value_type(b) != OPA_ARRAY)
    {
        return NULL;
    }

    opa_array_t *x = opa_cast_array(a);
    opa_array_t *y = opa_cast_array(b);
    opa_array_t *r = opa_cast_array(opa_array_with_cap(x->len + y->len));

    for (int i = 0; i < x->len; i++)
    {
        opa_array_append(r, x->elems[i].v);
    }

    for (int i = 0; i < y->len; i++)
    {
        opa_array_append(r, y->elems[i].v);
    }

    return &r->hdr;
}

OPA_BUILTIN
opa_value *opa_array_slice(opa_value *a, opa_value *i, opa_value *j)
{
    if (opa_value_type(a) != OPA_ARRAY || opa_value_type(i) != OPA_NUMBER || opa_value_type(j) != OPA_NUMBER)
    {
        return NULL;
    }

    opa_array_t *arr = opa_cast_array(a);
    long long start;
    long long stop;

    if (opa_number_try_int(opa_cast_number(i), &start) != 0 ||
        opa_number_try_int(opa_cast_number(j), &stop) != 0)
    {
        return NULL;
    }

    if (stop < 0)
    {
        stop = 0;
    } else if (stop > arr->len) {
        stop = arr->len;
    }

    if (start < 0) {
        start = 0;
    } else if (start > stop) {
        start = stop;
    }

    opa_array_t *r = opa_cast_array(opa_array_with_cap(stop-start));

    for (int i = start; i < stop; i++)
    {
        opa_array_append(r, arr->elems[i].v);
    }

    return &r->hdr;
}

OPA_BUILTIN
opa_value *opa_array_reverse(opa_value *a)
{
    if (opa_value_type(a) != OPA_ARRAY)
    {
        return NULL;
    }

    opa_array_t *arr = opa_cast_array(a);

    opa_array_t *reversed = opa_cast_array(opa_array_with_cap(arr->len));

    int n = arr->len;

    for (int i = 0; i < n; i++) {
        opa_array_append(reversed, arr->elems[n - 1 - i].v);
    }

    return &reversed->hdr;
}
