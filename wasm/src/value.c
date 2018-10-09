#include "malloc.h"
#include "string.h"
#include "value.h"

#define OPA_ARRAY_INITIAL_CAP (10)

opa_value *opa_value_get_object(opa_object_t *obj, opa_value *key)
{
    opa_object_elem_t *elem = opa_object_get(obj, key);

    if (elem != NULL)
    {
        return elem->v;
    }

    return NULL;
}

opa_value *opa_value_get_array_native(opa_array_t *arr, long long i)
{
    if (i >= arr->len)
    {
        return NULL;
    }

    return arr->elems[i].v;
}

opa_value *opa_value_get_array(opa_array_t *arr, opa_value *key)
{
    if (key->type != OPA_NUMBER)
    {
        return NULL;
    }

    opa_number_t *num = opa_cast_number(key);

    if (num->is_float)
    {
        return NULL;
    }

    if (num->v.i < 0)
    {
        return NULL;
    }

    return opa_value_get_array_native(arr, num->v.i);
}

opa_value *opa_value_get(opa_value *node, opa_value *key)
{
    if (node != NULL)
    {
        switch (node->type)
        {
        case OPA_ARRAY:
            return opa_value_get_array(opa_cast_array(node), key);
        case OPA_OBJECT:
            return opa_value_get_object(opa_cast_object(node), key);
        }
    }
    return NULL;
}

opa_value *opa_value_iter_object(opa_object_t *obj, opa_value *prev)
{
    if (prev == NULL)
    {
        if (obj->head == NULL)
        {
            return NULL;
        }

        return obj->head->k;
    }

    opa_object_elem_t *elem = opa_object_get(obj, prev);

    if (elem != NULL && elem->next != NULL)
    {
        return elem->next->k;
    }

    return NULL;
}

opa_value *opa_value_iter_array(opa_array_t *arr, opa_value *prev)
{
    if (prev == NULL)
    {
        if (arr->len == 0)
        {
            return NULL;
        }

        return arr->elems[0].i;
    }

    if (prev->type != OPA_NUMBER)
    {
        return NULL;
    }

    opa_number_t *num = opa_cast_number(prev);

    if (num->is_float)
    {
        return NULL;
    }

    long long i = num->v.i;
    i++;

    if (i < 0 || i >= arr->len)
    {
        return NULL;
    }

    return arr->elems[i].i;
}

opa_value *opa_value_iter(opa_value *node, opa_value *prev)
{
    if (node != NULL)
    {
        switch (node->type)
        {
        case OPA_ARRAY:
            return opa_value_iter_array(opa_cast_array(node), prev);
        case OPA_OBJECT:
            return opa_value_iter_object(opa_cast_object(node), prev);
        }
    }

    return NULL;
}

size_t opa_value_length_object(opa_object_t *obj)
{
    size_t i = 0;
    for (opa_object_elem_t *elem = obj->head; elem != NULL; elem = elem->next)
    {
        i++;
    }
    return i;
}

size_t opa_value_length_array(opa_array_t *arr)
{
    return arr->len;
}

size_t opa_value_length_string(opa_string_t *str)
{
    return str->len;
}

size_t opa_value_length(opa_value *node)
{
    switch (node->type)
    {
    case OPA_ARRAY:
        return opa_value_length_array(opa_cast_array(node));
    case OPA_OBJECT:
        return opa_value_length_object(opa_cast_object(node));
    case OPA_STRING:
        return opa_value_length_string(opa_cast_string(node));
    default:
        return 0;
    }
}

int opa_value_compare_float(double a, double b)
{
    if (a < b)
    {
        return -1;
    }
    else if (a > b)
    {
        return 1;
    }
    return 0;
}

int opa_value_compare_number(opa_number_t *a, opa_number_t *b)
{
    if (!a->is_float && !b->is_float)
    {
        if (a->v.i < b->v.i)
        {
            return -1;
        }
        else if (a->v.i > b->v.i)
        {
            return 1;
        }
        return 0;
    }

    if (a->is_float && b->is_float)
    {
        return opa_value_compare_float(a->v.f, b->v.f);
    }

    double a1, b1;

    if (!a->is_float)
    {
        a1 = (double)a->v.i;
        b1 = b->v.f;
    }
    else
    {
        a1 = a->v.f;
        b1 = (double)b->v.i;
    }

    return opa_value_compare_float(a1, b1);
}

int opa_value_compare_string(opa_string_t *a, opa_string_t *b)
{
    size_t min = a->len;

    if (b->len < min)
    {
        min = b->len;
    }

    int cmp = opa_strncmp(a->v, b->v, min);

    if (cmp != 0)
    {
        return cmp;
    }

    if (a->len < b->len)
    {
        return -1;
    }
    else if (a->len > b->len)
    {
        return 1;
    }
    return 0;
}

int opa_value_compare_array(opa_array_t *a, opa_array_t *b)
{
    size_t a_len = opa_value_length_array(a);
    size_t b_len = opa_value_length_array(b);

    size_t min = a_len;

    if (b_len < min)
    {
        min = b_len;
    }

    for (long long i = 0; i < min; i++)
    {
        opa_value *e1 = opa_value_get_array_native(a, i);
        opa_value *e2 = opa_value_get_array_native(b, i);
        int cmp = opa_value_compare(e1, e2);

        if (cmp != 0)
        {
            return cmp;
        }
    }

    if (a_len < b_len)
    {
        return -1;
    }
    else if (a_len > b_len)
    {
        return 1;
    }
    return 0;
}

int opa_value_compare_object(opa_object_t *a, opa_object_t *b)
{
    opa_array_t *a_keys = opa_object_keys(a);
    opa_array_t *b_keys = opa_object_keys(b);
    size_t a_len = opa_value_length_array(a_keys);
    size_t b_len = opa_value_length_array(b_keys);
    size_t min = a_len;

    if (b_len < min)
    {
        min = b_len;
    }

    int cmp;

    for (size_t i = 0; i < min; i++)
    {
        cmp = opa_value_compare(a_keys->elems[i].v, b_keys->elems[i].v);

        if (cmp != 0)
        {
            goto finish;
        }

        opa_value *a_val = opa_value_get_object(a, a_keys->elems[i].v);
        opa_value *b_val = opa_value_get_object(b, b_keys->elems[i].v);

        cmp = opa_value_compare(a_val, b_val);

        if (cmp != 0)
        {
            goto finish;
        }
    }

    if (a_len < b_len)
    {
        return -1;
    }
    else if (a_len > b_len)
    {
        return 1;
    }

finish:
    opa_array_free(a_keys);
    opa_array_free(b_keys);
    return cmp;
}

int opa_value_compare(opa_value *a, opa_value *b)
{
    if (a == NULL && b == NULL)
    {
        return 0;
    }
    else if (b == NULL)
    {
        return 1;
    }
    else if (a == NULL)
    {
        return -1;
    }

    if (a->type < b->type)
    {
        return -1;
    }
    else if (b->type < a->type)
    {
        return 1;
    }

    switch (a->type)
    {
    case OPA_NULL:
        return 0;
    case OPA_BOOLEAN:
    {
        opa_boolean_t *a1 = opa_cast_boolean(a);
        opa_boolean_t *b1 = opa_cast_boolean(b);
        return a1->v - b1->v;
    }
    case OPA_NUMBER:
    {
        opa_number_t *a1 = opa_cast_number(a);
        opa_number_t *b1 = opa_cast_number(b);
        return opa_value_compare_number(a1, b1);
    }
    case OPA_STRING:
    {
        opa_string_t *a1 = opa_cast_string(a);
        opa_string_t *b1 = opa_cast_string(b);
        return opa_value_compare_string(a1, b1);
    }
    case OPA_ARRAY:
    {
        opa_array_t *a1 = opa_cast_array(a);
        opa_array_t *b1 = opa_cast_array(b);
        return opa_value_compare_array(a1, b1);
    }
    case OPA_OBJECT:
    {
        opa_object_t *a1 = opa_cast_object(a);
        opa_object_t *b1 = opa_cast_object(b);
        return opa_value_compare_object(a1, b1);
    }
    default:
    {
        opa_abort("illegal value");
        return 0;
    }
    }
}

int opa_value_not_equal(opa_value *a, opa_value *b)
{
    return opa_value_compare(a, b) != 0;
}

void opa_value_free(opa_value *node)
{
    switch (node->type)
    {
    case OPA_NULL:
        opa_free(node);
        return;
    case OPA_BOOLEAN:
        opa_free(opa_cast_boolean(node));
        return;
    case OPA_NUMBER:
        opa_free(opa_cast_number(node));
        return;
    case OPA_STRING:
        opa_free(opa_cast_string(node));
        return;
    case OPA_ARRAY:
        opa_array_free(opa_cast_array(node));
        return;
    case OPA_OBJECT:
        opa_object_free(opa_cast_object(node));
        return;
    }
}

int opa_value_boolean(opa_value *node)
{
    return opa_cast_boolean(node)->v;
}

long long opa_value_int(opa_value *node)
{
    return opa_cast_number(node)->v.i;
}

double opa_value_float(opa_value *node)
{
    return opa_cast_number(node)->v.f;
}

const char *opa_value_string(opa_value *node)
{
    return opa_cast_string(node)->v;
}

opa_value *opa_null()
{
    opa_value *ret = (opa_value *)opa_malloc(sizeof(opa_value));
    ret->type = OPA_NULL;
    return ret;
}

opa_value *opa_boolean(int v)
{
    opa_boolean_t *ret = (opa_boolean_t *)opa_malloc(sizeof(opa_boolean_t));
    ret->hdr.type = OPA_BOOLEAN;
    ret->v = v;
    return &ret->hdr;
}

opa_value *opa_number_int(long long v)
{
    opa_number_t *ret = (opa_number_t *)opa_malloc(sizeof(opa_number_t));
    ret->hdr.type = OPA_NUMBER;
    ret->is_float = 0;
    ret->v.i = v;
    return &ret->hdr;
}

opa_value *opa_number_float(double v)
{
    opa_number_t *ret = (opa_number_t *)opa_malloc(sizeof(opa_number_t));
    ret->hdr.type = OPA_NUMBER;
    ret->is_float = 1;
    ret->v.f = v;
    return &ret->hdr;
}

opa_value *opa_string(const char *v, size_t len)
{
    opa_string_t *ret = (opa_string_t *)opa_malloc(sizeof(opa_string_t));
    ret->hdr.type = OPA_STRING;
    ret->len = len;
    ret->v = v;
    return &ret->hdr;
}

opa_value *opa_string_terminated(const char *v)
{
    opa_string_t *ret = (opa_string_t *)opa_malloc(sizeof(opa_string_t));
    ret->hdr.type = OPA_STRING;
    ret->len = opa_strlen(v);
    ret->v = v;
    return &ret->hdr;
}

void __opa_array_grow(opa_array_t *arr)
{
    if (arr->cap == 0)
    {
        arr->cap = OPA_ARRAY_INITIAL_CAP;
    }
    else
    {
        arr->cap *= 2;
    }

    opa_array_elem_t *elems = (opa_array_elem_t *)opa_malloc(arr->cap * sizeof(opa_array_elem_t));

    for (int i = 0; i < arr->len; i++)
    {
        elems[i] = arr->elems[i];
    }

    opa_free(arr->elems);
    arr->elems = elems;
}

opa_value *opa_array()
{
    return opa_array_with_cap(0);
}

opa_value *opa_array_with_cap(size_t cap)
{
    opa_array_t *ret = (opa_array_t *)opa_malloc(sizeof(opa_array_t));
    ret->hdr.type = OPA_ARRAY;
    ret->len = 0;
    ret->cap = cap;
    ret->elems = NULL;

    if (ret->cap != 0)
    {
        __opa_array_grow(ret);
    }

    return &ret->hdr;
}

opa_value *opa_object()
{
    opa_object_t *ret = (opa_object_t *)opa_malloc(sizeof(opa_object_t));
    ret->hdr.type = OPA_OBJECT;
    return &ret->hdr;
}

void opa_value_boolean_set(opa_value *v, int b)
{
    opa_boolean_t *ret = opa_cast_boolean(v);
    ret->v = b;
}

void opa_array_free(opa_array_t *arr)
{
    if (arr->elems != NULL)
    {
        for (size_t i = 0; i < arr->len; i++)
        {
            opa_free(arr->elems[i].i);
        }

        opa_free(arr->elems);
    }

    opa_free(arr);
}

void opa_array_append(opa_array_t *arr, opa_value *v)
{
    if (arr->len >= arr->cap)
    {
        __opa_array_grow(arr);
    }

    size_t i = arr->len++;
    arr->elems[i].i = opa_number_int(i);
    arr->elems[i].v = v;
}

void opa_array_sort(opa_array_t *arr, opa_compare_fn cmp_fn)
{
    for (size_t i = 1; i < arr->len; i++)
    {
        opa_array_elem_t elem = arr->elems[i];
        size_t j = i - 1;

        while (j >= 0 && cmp_fn(arr->elems[j].v, elem.v) > 0)
        {
            arr->elems[j + 1] = arr->elems[j];
            j = j - 1;
        }

        arr->elems[j + 1] = elem;
    }
}

void opa_object_free(opa_object_t *obj)
{
    opa_object_elem_t *prev = NULL;

    for (opa_object_elem_t *curr = obj->head; curr != NULL; curr = curr->next)
    {
        if (prev != NULL)
        {
            opa_free(prev);
        }

        prev = curr;
    }

    if (prev != NULL)
    {
        opa_free(prev);
    }
}

opa_array_t *opa_object_keys(opa_object_t *obj)
{
    opa_array_t *ret = opa_cast_array(opa_array_with_cap(opa_value_length_object(obj)));
    opa_object_elem_t *elem = opa_object_iter(obj, NULL);

    while (elem != NULL)
    {
        opa_array_append(ret, elem->k);
        elem = opa_object_iter(obj, elem);
    }

    opa_array_sort(ret, opa_value_compare);
    return ret;
}

opa_object_elem_t *__opa_object_elem_alloc(opa_value *k, opa_value *v)
{
    opa_object_elem_t *elem = (opa_object_elem_t *)opa_malloc(sizeof(opa_object_elem_t));
    elem->next = NULL;
    elem->k = k;
    elem->v = v;
    return elem;
}

void opa_object_insert(opa_object_t *obj, opa_value *k, opa_value *v)
{
    opa_object_elem_t *curr = NULL;

    if (obj->head != NULL)
    {
        for (curr = obj->head; curr->next != NULL; curr = curr->next)
        {
            if (opa_value_compare(curr->k, k) == 0)
            {
                curr->v = v;
                return;
            }
        }
    }

    opa_object_elem_t *new = __opa_object_elem_alloc(k, v);

    if (curr != NULL)
    {
        curr->next = new;
    }
    else
    {
        obj->head = new;
    }
}

opa_object_elem_t *opa_object_get(opa_object_t *obj, opa_value *key)
{
    for (opa_object_elem_t *curr = obj->head; curr != NULL; curr = curr->next)
    {
        if (opa_value_compare(curr->k, key) == 0)
        {
            return curr;
        }
    }

    return NULL;
}

opa_object_elem_t *opa_object_iter(opa_object_t *obj, opa_object_elem_t *prev)
{
    if (prev == NULL)
    {
        return obj->head;
    }

    return prev->next;
}
