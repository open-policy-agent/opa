#include "object.h"

opa_value *builtin_object_filter(opa_value *obj, opa_value *keys)
{
    if (opa_value_type(obj) != OPA_OBJECT)
    {
        return NULL;
    }

    if (opa_value_type(keys) != OPA_OBJECT && opa_value_type(keys) != OPA_ARRAY &&
        opa_value_type(keys) != OPA_SET)
    {
        return NULL;
    }

    opa_object_t *r = opa_cast_object(opa_object());

    for (opa_value *key = opa_value_iter(keys, NULL); key != NULL;
         key = opa_value_iter(keys, key))
    {
        opa_value* k;
        switch (opa_value_type(keys))
        {
        case OPA_OBJECT:
        case OPA_SET:
            k = key;
            break;
        case OPA_ARRAY:
            k = opa_value_get(keys, key);
        }

        opa_object_elem_t *elem = opa_object_get(opa_cast_object(obj), k);
        if (elem != NULL)
        {
            opa_object_insert(r, k, elem->v);
        }
    }

    return &r->hdr;
}

opa_value *builtin_object_get(opa_value *obj, opa_value *key, opa_value *value)
{
    if (opa_value_type(obj) != OPA_OBJECT)
    {
        return NULL;
    }

    opa_object_elem_t *elem = opa_object_get(opa_cast_object(obj), key);
    if (elem != NULL)
    {
       return elem->v;
    }

    return value;
}
