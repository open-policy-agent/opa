#include "std.h"
#include "object.h"

static opa_value *__merge(opa_value *a, opa_value *b);
static opa_value *__merge_with_overwrite(opa_value *a, opa_value *b);
static void __copy_object_elem(opa_object_t *result, opa_value *a, opa_value *b);
opa_array_t *__get_json_paths(opa_value *a);
opa_object_t *__paths_to_object(opa_value *a);
opa_array_t *__parse_path(opa_value *a);
opa_value *__json_remove(opa_value *a, opa_value *b);
opa_value *__json_filter(opa_value *a, opa_value *b);

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

opa_object_t *__paths_to_object(opa_value *a)
{
    opa_object_t *root = opa_cast_object(opa_object());
    opa_array_t *paths = opa_cast_array(a);

    for (int i = 0; i < paths->len; i++)
    {
        opa_object_t *node = root;
        uint32_t done = 0;

        opa_array_t *terms = opa_cast_array(paths->elems[i].v);

        for (int j = 0; j < terms->len - 1 && !done; j++) {

            opa_value *k = terms->elems[j].v;

            opa_value *child = opa_value_get(&node->hdr, k);

            if (child == NULL) {
                opa_object_t *obj = opa_cast_object(opa_object());
                opa_object_insert(node, k, &obj->hdr);
                node = obj;
                continue;
            }

            switch (opa_value_type(child))
            {
                case OPA_NULL:
                    done = 1;
                    break;
                case OPA_OBJECT:
                    node = opa_cast_object(child);
            }
        }

        if (!done)
        {
            opa_value *key = terms->elems[terms->len - 1].v;
            opa_object_insert(node, key, opa_null());
        }
    }

    return root;

}

opa_array_t *__parse_path(opa_value *a)
{
    // paths can either be a `/` separated json path or
    // an array or set of values

    opa_array_t *path_segments = opa_cast_array(opa_array());

    switch (opa_value_type(a))
    {
        case OPA_STRING:
        {
            opa_string_t *x = opa_cast_string(a);

            if (x->len == 0)
            {
                return path_segments;
            }

            opa_value *s = opa_strings_split(opa_strings_trim_left(a, opa_string_terminated("/")), opa_string_terminated("/"));

            opa_array_t *parts = opa_cast_array(s);

            for (int i = 0; i < parts->len; i++)
            {
                opa_value *s = opa_strings_replace(parts->elems[i].v, opa_string_terminated("~1"), opa_string_terminated("/"));
                opa_value *part = opa_strings_replace(s, opa_string_terminated("~0"), opa_string_terminated("~"));
                opa_array_append(path_segments, part);
            }

            return path_segments;
        }
        case OPA_ARRAY:
        {
            opa_array_t *y = opa_cast_array(a);

            for (int i = 0; i < y->len; i++)
            {
                opa_array_append(path_segments, y->elems[i].v);
            }

            return path_segments;
        }
    }

    return NULL;
}

opa_array_t *__get_json_paths(opa_value *a)
{
    opa_array_t *paths = opa_cast_array(opa_array());

    for (opa_value *key = opa_value_iter(a, NULL); key != NULL; key = opa_value_iter(a, key))
    {
        opa_value* k;
        switch (opa_value_type(a))
        {
        case OPA_SET:
            k = key;
            break;
        case OPA_ARRAY:
            k = opa_value_get(a, key);
        }

        opa_array_t* path = __parse_path(k);

        if (path == NULL)
        {
            return NULL;
        }
        opa_array_append(paths, &path->hdr);
    }

    return paths;
}

opa_value *__json_remove(opa_value *a, opa_value *b)
{
    if (b == NULL)
    {
        // The paths diverged, return a
        return a;
    }

    opa_value* bObj;
    switch (opa_value_type(b))
    {
        case OPA_OBJECT:
        {
            bObj = b;
            break;
        }
        case OPA_NULL:
        {
            // Means we hit a leaf node on "b", dont add the value for a
            return NULL;
        }
        default:
            // The paths diverged, return a
            return a;
    }

    switch (opa_value_type(a))
    {
        case OPA_STRING:
        case OPA_NUMBER:
        case OPA_BOOLEAN:
        case OPA_NULL:
        {
            return a;
        }
        case OPA_OBJECT:
        {
            opa_object_t *new_obj = opa_cast_object(opa_object());

            for (opa_value *key = opa_value_iter(a, NULL); key != NULL; key = opa_value_iter(a, key))
            {
                opa_value *value = opa_value_get(a, key);
                opa_value *diff_value = __json_remove(value, opa_value_get(bObj, key));

                if (diff_value != NULL)
                {
                    opa_object_insert(new_obj, key, diff_value);
                }
            }
            return &new_obj->hdr;
        }
        case OPA_SET:
        {
            opa_set_t *new_set = opa_cast_set(opa_set());
            opa_set_t *set = opa_cast_set(a);

            for (int i = 0; i < set->n; i++)
            {
                opa_set_elem_t *elem = set->buckets[i];

                while (elem != NULL)
                {
                    opa_value *diff_value = __json_remove(elem->v, opa_value_get(bObj, elem->v));

                    if (diff_value != NULL)
                    {
                        opa_set_add(new_set, diff_value);
                    }
                    elem = elem->next;
                }
            }
            return &new_set->hdr;
        }
        case OPA_ARRAY:
        {
            opa_array_t *new_array = opa_cast_array(opa_array());
            opa_array_t *array = opa_cast_array(a);

            for (int i = 0; i < array->len; i++)
            {
                opa_value *value = array->elems[i].v;

                opa_value *diff_value = __json_remove(value, opa_value_get(bObj, opa_strings_format_int(opa_number_int(i), opa_number_int(10))));

                if (diff_value != NULL)
                {
                    opa_array_append(new_array, diff_value);
                }
            }
            return &new_array->hdr;
        }
    }

    return NULL;
}

opa_value *__json_filter(opa_value *a, opa_value *b)
{

    if (opa_value_compare(b, opa_null()) == 0)
    {
        return a;
    }

    if (opa_value_type(b) != OPA_OBJECT)
    {
        return NULL;
    }

    switch (opa_value_type(a))
    {
        case OPA_STRING:
        case OPA_NUMBER:
        case OPA_BOOLEAN:
        case OPA_NULL:
        {
            return a;
        }
        case OPA_OBJECT:
        {
            opa_object_t *new_obj = opa_cast_object(opa_object());

            opa_object_t *iter_obj = opa_cast_object(a);
            opa_object_t *other = opa_cast_object(b);

            if (iter_obj->len < other->len)
            {
                iter_obj = opa_cast_object(b);
                other = opa_cast_object(a);
            }

            for (opa_value *key = opa_value_iter(&iter_obj->hdr, NULL); key != NULL; key = opa_value_iter(&iter_obj->hdr, key))
            {

                if (opa_value_get(&other->hdr, key) != NULL)
                {
                    opa_value *filtered_value = __json_filter(opa_value_get(a, key), opa_value_get(b, key));

                    if (filtered_value != NULL)
                    {
                        opa_object_insert(new_obj, key, filtered_value);
                    }
                }
            }
            return &new_obj->hdr;
        }
        case OPA_SET:
        {
            opa_set_t *new_set = opa_cast_set(opa_set());
            opa_set_t *set = opa_cast_set(a);

            for (int i = 0; i < set->n; i++)
            {
                opa_set_elem_t *elem = set->buckets[i];

                while (elem != NULL)
                {
                    opa_value *filtered_value = __json_filter(elem->v, opa_value_get(b, elem->v));

                    if (filtered_value != NULL)
                    {
                        opa_set_add(new_set, filtered_value);
                    }
                    elem = elem->next;
                }
            }
            return &new_set->hdr;
        }
        case OPA_ARRAY:
        {
            opa_array_t *new_array = opa_cast_array(opa_array());
            opa_array_t *array = opa_cast_array(a);

            for (int i = 0; i < array->len; i++)
            {
                opa_value *value = array->elems[i].v;

                opa_value *filtered_value = __json_filter(value, opa_value_get(b, opa_strings_format_int(opa_number_int(i), opa_number_int(10))));

                if (filtered_value != NULL)
                {
                    opa_array_append(new_array, filtered_value);
                }
            }
            return &new_array->hdr;
        }
    }

    return NULL;
}

OPA_BUILTIN
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

    for (opa_value *key = opa_value_iter(keys, NULL); key != NULL; key = opa_value_iter(keys, key))
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

OPA_BUILTIN
opa_value *builtin_object_get(opa_value *obj, opa_value *key, opa_value *value)
{
    if (opa_value_type(obj) != OPA_OBJECT)
    {
        return NULL;
    }

    opa_value *elem;

    // if the key is not an array, then we get that top level key from the object/array or return the default value
    if (opa_value_type(key) != OPA_ARRAY) {
        elem = opa_value_get(obj, key);
        if (elem != NULL)
        {
            return elem;
        }

        return value;
    }

    size_t path_len = opa_cast_array(key)->len;
    // if the path is empty, then we skip selecting nested keys and return the default
    if (path_len == 0) {
        return obj;
    }

    for (int i = 0; i < path_len; i++)
    {
        opa_value *path_component = opa_cast_array(key)->elems[i].v;

        elem = opa_value_get(obj, path_component);

        if (elem == NULL)
        {
            return value;
        }

        if (i == path_len-1)
        {
            return elem;
        }

        obj = elem;
    }

    return value;
}

OPA_BUILTIN
opa_value *builtin_object_keys(opa_value *a)
{
    if (opa_value_type(a) != OPA_OBJECT)
    {
        return NULL;
    }

    opa_object_t *obj = opa_cast_object(a);
    opa_set_t *keys = opa_cast_set(opa_set_with_cap(obj->len));

    for (int i = 0; i < obj->n; i++)
    {
        opa_object_elem_t *elem = obj->buckets[i];

        while (elem != NULL)
        {
            opa_set_add(keys, elem->k);
            elem = elem->next;
        }
    }

    return &keys->hdr;
}

OPA_BUILTIN
opa_value *builtin_object_remove(opa_value *obj, opa_value *keys)
{
    if (opa_value_type(obj)  != OPA_OBJECT ||
        (opa_value_type(keys) != OPA_OBJECT &&
         opa_value_type(keys) != OPA_ARRAY &&
         opa_value_type(keys) != OPA_SET))
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

OPA_BUILTIN
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

OPA_BUILTIN
opa_value *builtin_json_remove(opa_value *obj, opa_value *paths)
{
    if (opa_value_type(obj) != OPA_OBJECT)
    {
        return NULL;
    }

    if (opa_value_type(paths) != OPA_ARRAY && opa_value_type(paths) != OPA_SET)
    {
        return NULL;
    }

    // Build a list of json pointers to remove
    opa_array_t *json_paths = __get_json_paths(paths);

    if (json_paths == NULL)
    {
        return NULL;
    }

    opa_object_t *json_paths_obj = __paths_to_object(&json_paths->hdr);

    opa_value *r = __json_remove(obj, &json_paths_obj->hdr);

    return r;
}

OPA_BUILTIN
opa_value *builtin_json_filter(opa_value *obj, opa_value *paths)
{
    if (opa_value_type(obj) != OPA_OBJECT)
    {
        return NULL;
    }

    if (opa_value_type(paths) != OPA_ARRAY && opa_value_type(paths) != OPA_SET)
    {
        return NULL;
    }

    // Build a list of filter strings
    opa_array_t *json_paths = __get_json_paths(paths);

    if (json_paths == NULL)
    {
        return NULL;
    }

    opa_object_t *json_paths_obj = __paths_to_object(&json_paths->hdr);

    opa_value *r = __json_filter(obj, &json_paths_obj->hdr);

    return r;
}
