#include "malloc.h"
#include "mpd.h"
#include "std.h"
#include "str.h"
#include "string.h"
#include "template-string.h"
#include "encoding.h"
#include "json.h"

size_t to_string(opa_value *v, opa_value **out)
{
    if (v == NULL) {
        return -1;
    }

    if (opa_value_type(v) == OPA_SET) {
        opa_set_t *set = opa_cast_set(v);
        if (set->len == 0) {
            *out = opa_string_terminated("<undefined>");
            return 11;
        }
        if (set->len > 1) {
            return -1;
        }

        v = NULL;
        // There might be multiple buckets ...
        for (int i = 0; i < set->n; i++) {
            opa_set_elem_t *bucket = set->buckets[i];
            if (bucket->v != NULL) {
                // ... but we expect a single entry
                v = bucket->v;
                break;
            }
        }
    }

    if (v == NULL) {
        return -1;
    }

    if (opa_value_type(v) == OPA_STRING) {
        *out = v;
        opa_string_t *str = opa_cast_string(v);
        return str->len;
    }

    char *str = opa_value_dump(v);
    size_t len = opa_strlen(str);
    *out = opa_string_allocated(str, len);
    return len;
}

OPA_BUILTIN
opa_value *opa_template_string(opa_value *a)
{
    if (opa_value_type(a) != OPA_ARRAY)
    {
        return NULL;
    }

    opa_array_t *parts = opa_cast_array(a);
    opa_array_t *result = opa_cast_array(opa_array_with_cap(parts->len));

    size_t len = 1; // 1 for '\0'
    for (int i = 0; i < parts->len; i++)
    {
        opa_value *v = NULL;
        size_t l = to_string(parts->elems[i].v, &v);
        if (l == -1) {
            // Invalid element; likely multiple values.
            return NULL;
        }
        len += l;
        opa_array_append(result, v);
    }

    char *buf = opa_malloc(len);
    size_t j = 0;

    for (int i = 0; i < result->len; i++)
    {
        opa_string_t *part_str = opa_cast_string(result->elems[i].v);
        memcpy(&buf[j], part_str->v, part_str->len);
        j += part_str->len;
    }

    buf[len - 1] = '\0';
    return opa_string_allocated(buf, len - 1);
}
