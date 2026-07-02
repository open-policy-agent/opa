#include "set.h"
#include "std.h"

OPA_BUILTIN
opa_value *opa_set_diff(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_SET || opa_value_type(b) != OPA_SET)
    {
        return NULL;
    }

    opa_set_t *x = opa_cast_set(a);
    opa_set_t *y = opa_cast_set(b);
    opa_set_t *r = opa_cast_set(opa_set());

    for (int i = 0; i < x->n; i++)
    {
        opa_set_elem_t *elem = x->buckets[i];

        while (elem != NULL)
        {
            if (opa_set_get(y, elem->v) == NULL)
            {
                opa_set_add(r, elem->v);
            }
            elem = elem->next;
        }
    }

	return &r->hdr;
}

OPA_BUILTIN
opa_value *opa_set_intersection(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_SET || opa_value_type(b) != OPA_SET)
    {
        return NULL;
    }

    opa_set_t *x = opa_cast_set(a);
    opa_set_t *y = opa_cast_set(b);
    opa_set_t *r = opa_cast_set(opa_set_with_cap(x->len < y->len ? x->len : y->len));

    if (y->len < x->len)
    {
        x = opa_cast_set(b);
        y = opa_cast_set(a);
    }

    for (int i = 0; i < x->n; i++)
    {
        opa_set_elem_t *elem = x->buckets[i];

        while (elem != NULL)
        {
            if (opa_set_get(y, elem->v) != NULL)
            {
                opa_set_add(r, elem->v);
            }
            elem = elem->next;
        }
    }

    return &r->hdr;
}

OPA_BUILTIN
opa_value *opa_sets_intersection(opa_value *v)
{
    if (opa_value_type(v) != OPA_SET)
    {
        return NULL;
    }

    opa_set_t *s = opa_cast_set(v);

    if (s->len == 0)
    {
        return opa_set();
    }

    opa_value *r = NULL;

    for (int i = 0; i < s->n; i++)
    {
        opa_set_elem_t *elem = s->buckets[i];

        while (elem != NULL)
        {
            if (opa_value_type(elem->v) != OPA_SET)
            {
                return NULL;
            }

            if (r == NULL)
            {
                r = opa_set_union(opa_set(), elem->v);
            } else {
                opa_value *x = opa_set_intersection(r, elem->v);
                opa_value_free_shallow(r);
                if (x == NULL)
                {
                    return NULL;
                }

                r = x;
            }

            elem = elem->next;
        }
    }

    return r;
}

OPA_BUILTIN
opa_value *opa_set_union(opa_value *a, opa_value *b)
{
    if (opa_value_type(a) != OPA_SET || opa_value_type(b) != OPA_SET)
    {
        return NULL;
    }

    opa_set_t *x = opa_cast_set(a);
    opa_set_t *y = opa_cast_set(b);
    opa_set_t *r = opa_cast_set(opa_set());

    for (int i = 0; i < x->n; i++)
    {
        opa_set_elem_t *elem = x->buckets[i];

        while (elem != NULL)
        {
            opa_set_add(r, elem->v);
            elem = elem->next;
        }
    }

    for (int i = 0; i < y->n; i++)
    {
        opa_set_elem_t *elem = y->buckets[i];

        while (elem != NULL)
        {
            opa_set_add(r, elem->v);
            elem = elem->next;
        }
    }

	return &r->hdr;
}

OPA_BUILTIN
opa_value *opa_sets_union(opa_value *v)
{
    if (opa_value_type(v) != OPA_SET)
    {
        return NULL;
    }

    opa_set_t *s = opa_cast_set(v);
    opa_value *r = opa_set();

    for (int i = 0; i < s->n; i++)
    {
        opa_set_elem_t *elem = s->buckets[i];

        while (elem != NULL)
        {
            if (opa_value_type(elem->v) != OPA_SET)
            {
                return NULL;
            }

            opa_value *x = opa_set_union(r, elem->v);
            opa_value_free_shallow(r);
            if (x == NULL)
            {
                return NULL;
            }

            r = x;
            elem = elem->next;
        }
    }

    return r;
}
