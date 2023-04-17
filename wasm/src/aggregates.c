#include "aggregates.h"
#include "mpd.h"
#include "std.h"
#include "unicode.h"
#include "re2/util/utf.h"

OPA_BUILTIN
opa_value *opa_agg_count(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_STRING: {
        opa_string_t *s = opa_cast_string(v);
        int units = 0;
        Rune rune;

        for (int i = 0, len = 0; i < s->len; i += len)
        {
            len = chartorune(&rune, &s->v[i]);
            units++;
        }

        return opa_number_int(units);
    }
    case OPA_ARRAY:
        return opa_number_int(opa_cast_array(v)->len);
    case OPA_OBJECT:
        return opa_number_int(opa_cast_object(v)->len);
    case OPA_SET:
        return opa_number_int(opa_cast_set(v)->len);
    default:
        return NULL;
    }
}

static mpd_t *mpd_int(int v)
{
    mpd_t *r = mpd_qnew();
    uint32_t status = 0;
    mpd_qset_i32(r, v, mpd_max_ctx(), &status);
    if (status)
    {
        opa_abort("aggregates: int");
    }

    return r;
}

OPA_BUILTIN
opa_value *opa_agg_sum(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_ARRAY: {
        opa_array_t *a = opa_cast_array(v);
        mpd_t *r = mpd_int(0);

        for (int i = 0; i < a->len; i++)
        {
            if (opa_value_type(a->elems[i].v) != OPA_NUMBER)
            {
                mpd_del(r);
                return NULL;
            }

            r = qadd(r, opa_number_to_bf(a->elems[i].v));
        }

        return opa_bf_to_number(r);
    }

    case OPA_SET: {
        opa_set_t *s = opa_cast_set(v);
        mpd_t *r = mpd_int(0);

        for (int i = 0; i < s->n; i++)
        {
            for (opa_set_elem_t *elem = s->buckets[i]; elem != NULL; elem = elem->next)
            {
                if (opa_value_type(elem->v) != OPA_NUMBER)
                {
                    mpd_del(r);
                    return NULL;
                }

                r = qadd(r, opa_number_to_bf(elem->v));
            }
        }

        return opa_bf_to_number(r);
    }

    default:
        return NULL;
    }
}

OPA_BUILTIN
opa_value *opa_agg_product(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_ARRAY: {
        opa_array_t *a = opa_cast_array(v);
        mpd_t *r = mpd_int(1);

        for (int i = 0; i < a->len; i++)
        {
            if (opa_value_type(a->elems[i].v) != OPA_NUMBER)
            {
                mpd_del(r);
                return NULL;
            }

            r = qmul(r, opa_number_to_bf(a->elems[i].v));
        }

        return opa_bf_to_number(r);
    }

    case OPA_SET: {
        opa_set_t *s = opa_cast_set(v);
        mpd_t *r = mpd_int(1);

        for (int i = 0; i < s->n; i++)
        {
            for (opa_set_elem_t *elem = s->buckets[i]; elem != NULL; elem = elem->next)
            {
                if (opa_value_type(elem->v) != OPA_NUMBER)
                {
                    mpd_del(r);
                    return NULL;
                }

                r = qmul(r, opa_number_to_bf(elem->v));
            }
        }

        return opa_bf_to_number(r);
    }

    default:
        return NULL;
    }
}

OPA_BUILTIN
opa_value *opa_agg_max(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_ARRAY: {
        opa_array_t *a = opa_cast_array(v);
        opa_value *max = NULL;

        for (int i = 0; i < a->len; i++)
        {
            if (max == NULL || opa_value_compare(max, a->elems[i].v) < 0)
            {
                max = a->elems[i].v;
            }
        }

        return max;
    }

    case OPA_SET: {
        opa_set_t *s = opa_cast_set(v);
        if (s->len == 0)
        {
            return NULL;
        }

        opa_value *max = NULL;
        for (int i = 0; i < s->n; i++)
        {
            for (opa_set_elem_t *elem = s->buckets[i]; elem != NULL; elem = elem->next)
            {
                if (max == NULL || opa_value_compare(max, elem->v) < 0)
                {
                    max = elem->v;
                }
            }
        }

        return max;
    }

    default:
        return NULL;
    }
}

OPA_BUILTIN
opa_value *opa_agg_min(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_ARRAY: {
        opa_array_t *a = opa_cast_array(v);
        opa_value *min = NULL;

        for (int i = 0; i < a->len; i++)
        {
            if (min == NULL || opa_value_compare(min, a->elems[i].v) > 0)
            {
                min = a->elems[i].v;
            }
        }

        return min;
    }

    case OPA_SET: {
        opa_set_t *s = opa_cast_set(v);
        if (s->len == 0)
        {
            return NULL;
        }

        opa_value *min = NULL;
        for (int i = 0; i < s->n; i++)
        {
            for (opa_set_elem_t *elem = s->buckets[i]; elem != NULL; elem = elem->next)
            {
                if (min == NULL || opa_value_compare(min, elem->v) > 0)
                {
                    min = elem->v;
                }
            }
        }

        return min;
    }

    default:
        return NULL;
    }
}

OPA_BUILTIN
opa_value *opa_agg_sort(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_ARRAY: {
        opa_value *r = opa_value_shallow_copy(v);
        opa_array_sort(opa_cast_array(r), opa_value_compare);
        return r;
    }
    case OPA_SET: {
        opa_set_t *s = opa_cast_set(v);
        opa_array_t *r = opa_cast_array(opa_array_with_cap(s->len));

        for (int i = 0; i < s->n; i++)
        {
            for (opa_set_elem_t *elem = s->buckets[i]; elem != NULL; elem = elem->next)
            {
                opa_array_append(r, elem->v);
            }
        }

        opa_array_sort(r, opa_value_compare);
        return &r->hdr;
    }
    default:
        return NULL;
    }
}

OPA_BUILTIN
opa_value *opa_agg_all(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_ARRAY: {
        opa_array_t *a = opa_cast_array(v);

        for (int i = 0; i < a->len; i++)
        {
            if (opa_value_type(a->elems[i].v) != OPA_BOOLEAN || opa_cast_boolean(a->elems[i].v)->v == false)
            {
                return opa_boolean(false);
            }
        }

        return opa_boolean(true);
    }
    case OPA_SET: {
        opa_set_t *s = opa_cast_set(v);

        for (int i = 0; i < s->n; i++)
        {
            for (opa_set_elem_t *elem = s->buckets[i]; elem != NULL; elem = elem->next)
            {
                if (opa_value_type(elem->v) != OPA_BOOLEAN || opa_cast_boolean(elem->v)->v == false)
                {
                    return opa_boolean(false);
                }
            }
        }

        return opa_boolean(true);
    }
    default:
        return NULL;
    }
}

OPA_BUILTIN
opa_value *opa_agg_any(opa_value *v)
{
    switch (opa_value_type(v))
    {
    case OPA_ARRAY: {
        opa_array_t *a = opa_cast_array(v);

        for (int i = 0; i < a->len; i++)
        {
            if (opa_value_type(a->elems[i].v) == OPA_BOOLEAN && opa_cast_boolean(a->elems[i].v)->v == true)
            {
                return opa_boolean(true);
            }
        }

        return opa_boolean(false);
    }
    case OPA_SET: {
        opa_set_t *s = opa_cast_set(v);
        if (s->len == 0)
        {
            return opa_boolean(false);
        }

        opa_boolean_t b = { .hdr.type = OPA_BOOLEAN, .v = true};
        return opa_boolean(opa_set_get(s, &b.hdr) == NULL ? false : true);
    }
    default:
        return NULL;
    }
}

OPA_BUILTIN
opa_value *builtin_member(opa_value *v, opa_value *collection)
{
    opa_value *prev = NULL;
    opa_value *curr = NULL;
    while ((curr = opa_value_iter(collection, prev)) != NULL)
    {
        if (opa_value_compare(v, opa_value_get(collection, curr)) == 0)
        {
            return opa_boolean(true);
        }
        prev = curr;
    }
    return opa_boolean(false);
}

OPA_BUILTIN
opa_value *builtin_member3(opa_value *key, opa_value *val, opa_value *collection)
{
    switch (opa_value_type(collection))
    {
    case OPA_ARRAY:
    case OPA_OBJECT:
        return opa_boolean(opa_value_compare(val, opa_value_get(collection, key)) == 0);
    }
    return opa_boolean(false);
}