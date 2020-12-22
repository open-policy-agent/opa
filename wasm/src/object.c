#include "object.h"


static opa_value *__merge(opa_value *a, opa_value *b);
static opa_value *__merge_with_overwrite(opa_value *a, opa_value *b);
static void __copy_object_elem(opa_object_t *result, opa_value *a, opa_value *b);

opa_value *__merge(opa_value *a, opa_value *b)
{

    opa_object_t *merged = opa_cast_object(opa_object());
    opa_object_t *obj = opa_cast_object(a);
    opa_object_t *other = opa_cast_object(b);

    for (opa_value *key = opa_value_iter(a, NULL); key != NULL;
         key = opa_value_iter(a, key))
    {

        opa_object_elem_t *original = opa_object_get(obj, key);
        opa_object_elem_t *elem = opa_object_get(other, key);

        // The key didn't exist in other, keep the original value.
        if (elem == NULL)
        {
            opa_object_insert(merged, key, original->v);
            continue;
        }

        // The key exists in both, resolve the conflict.
        opa_value *merged_value = __merge_with_overwrite(original->v, elem->v);
        opa_object_insert(merged, key, merged_value);

    }

    // Copy in any values from other for keys that don't exist in obj.
    __copy_object_elem(merged, a, b);

    return &merged->hdr;
}

opa_value *__merge_with_overwrite(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_OBJECT || opa_value_type(b) != OPA_OBJECT)
    {
        // If we can't merge, stick with the right-hand value.
        return b;
    }

    return __merge(a, b);
}

static void __copy_object_elem(opa_object_t *result, opa_value *a, opa_value *b)
{
    opa_object_t *obj = opa_cast_object(b);

    for (int i = 0; i < obj->n; i++)
    {
        opa_object_elem_t *elem = obj->buckets[i];

        while (elem != NULL)
        {
            opa_value *other = opa_value_get(a, elem->k);

            if (other == NULL)
            {
                opa_object_insert(result, elem->k, elem->v);
            }

            elem = elem->next;
        }
    }
}

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

opa_value *builtin_object_remove(opa_value *obj, opa_value *keys)
{
    if (opa_value_type(obj) != OPA_OBJECT)
    {
        return NULL;
    }

    opa_set_t *keys_to_remove = opa_cast_set(opa_set());

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
        opa_set_add(keys_to_remove, k);
    }

    opa_object_t *r = opa_cast_object(opa_object());

    for (opa_value *key = opa_value_iter(obj, NULL); key != NULL;
         key = opa_value_iter(obj, key))
    {
        if (opa_set_get(keys_to_remove, key) == NULL)
        {
            opa_object_elem_t *elem = opa_object_get(opa_cast_object(obj), key);
            if (elem != NULL)
            {
                opa_object_insert(r, key, elem->v);
            }
        }
    }

    return &r->hdr;
}

opa_value *builtin_object_union(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_OBJECT)
    {
        return NULL;
    }

    if (opa_value_type(b) != OPA_OBJECT)
    {
        return NULL;
    }

    opa_value *r = __merge(a, b);

    return r;
}
