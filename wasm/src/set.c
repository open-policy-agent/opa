#include "value.h"

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
