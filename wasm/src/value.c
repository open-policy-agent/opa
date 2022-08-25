#include <mpdecimal.h>

#include "json.h"
#include "malloc.h"
#include "mpd.h"
#include "str.h"
#include "value.h"

#define OPA_ARRAY_INITIAL_CAP (10)
#define OPA_OBJECT_MIN_BUCKETS (8)
#define OPA_OBJECT_LOAD_FACTOR (0.7)
#define OPA_SET_MIN_BUCKETS (8)
#define OPA_SET_LOAD_FACTOR (0.7)

static opa_value *__opa_object_with_buckets(size_t buckets);
static opa_value *__opa_set_with_buckets(size_t buckets);
static opa_array_t *__opa_set_values(opa_set_t *set);
static void __opa_object_insert_elem(opa_object_t *obj, opa_object_elem_t *new, size_t hash);
static void __opa_set_add_elem(opa_set_t *set, opa_set_elem_t *new, size_t hash);

OPA_INTERNAL
int opa_value_type(opa_value *node)
{
    // For all intents and purposes, interned strings are strings,
    // interned booleans are booleans.
    // Only opa_value_free and opa_value_shallow_copy handle them
    // separately, by refering to node->type directly.
    switch (node->type)
    {
    case OPA_STRING_INTERNED:
        return OPA_STRING;
    case OPA_BOOLEAN_INTERNED:
        return OPA_BOOLEAN;
    default:
        return node->type;
    }
}

opa_value *opa_value_get_object(opa_object_t *obj, opa_value *key)
{
    opa_object_elem_t *elem = opa_object_get(obj, key);
    return elem == NULL ? NULL : elem->v;
}

opa_value *opa_value_get_set(opa_set_t *set, opa_value *key)
{
    opa_set_elem_t *elem = opa_set_get(set, key);
    return elem == NULL ? NULL : elem->v;
}

opa_value *opa_value_get_array_native(opa_array_t *arr, long long i)
{
    return i >= arr->len ? NULL : arr->elems[i].v;
}

opa_value *opa_value_get_array(opa_array_t *arr, opa_value *key)
{
    if (key->type != OPA_NUMBER)
    {
        return NULL;
    }

    opa_number_t *num = opa_cast_number(key);

    long long i;

    if (opa_number_try_int(num, &i) != 0)
    {
        return NULL;
    }

    if (i < 0)
    {
        return NULL;
    }

    return opa_value_get_array_native(arr, i);
}

OPA_INTERNAL
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
        case OPA_SET:
            return opa_value_get_set(opa_cast_set(node), key);
        }
    }
    return NULL;
}

opa_object_elem_t *__opa_object_next_bucket(opa_object_t *obj, size_t i)
{
    for (; i < obj->n; i++) {
        opa_object_elem_t *elem = obj->buckets[i];
        if (elem != NULL) {
            return elem;
        }
    }

    return NULL;
}

opa_object_elem_t *__opa_object_get_bucket_elem(opa_object_elem_t *bucket, opa_value *key) {
    for (opa_object_elem_t *curr = bucket; curr != NULL; curr = curr->next)
    {
        if (opa_value_compare(curr->k, key) == 0)
        {
            return curr;
        }
    }

    return NULL;
}

opa_value *opa_value_iter_object(opa_object_t *obj, opa_value *prev)
{
    if (prev == NULL)
    {
        opa_object_elem_t *first = __opa_object_next_bucket(obj, 0);
        if (first != NULL) {
            return first->k;
        }

        return NULL;
    }

    size_t i = opa_value_hash(prev) % obj->n;
    opa_object_elem_t *elem = __opa_object_get_bucket_elem(obj->buckets[i], prev);
    opa_object_elem_t *next = elem->next;
    if (next != NULL) {
        return next->k;
    }

    next = __opa_object_next_bucket(obj, i+1);
    if (next != NULL) {
            return next->k;
    }

    return NULL;
}

opa_set_elem_t *__opa_set_next_bucket(opa_set_t *set, size_t i)
{
    for (; i < set->n; i++) {
        opa_set_elem_t *elem = set->buckets[i];
        if (elem != NULL) {
            return elem;
        }
    }

    return NULL;
}

opa_set_elem_t *__opa_set_get_bucket_elem(opa_set_elem_t *bucket, opa_value *v) {
    for (opa_set_elem_t *curr = bucket; curr != NULL; curr = curr->next)
    {
        if (opa_value_compare(curr->v, v) == 0)
        {
            return curr;
        }
    }

    return NULL;
}

opa_value *opa_value_iter_set(opa_set_t *set, opa_value *prev)
{
    if (prev == NULL)
    {
        opa_set_elem_t *first = __opa_set_next_bucket(set, 0);
        if (first != NULL) {
            return first->v;
        }

        return NULL;
    }

    size_t i = opa_value_hash(prev) % set->n;
    opa_set_elem_t *elem = __opa_set_get_bucket_elem(set->buckets[i], prev);
    opa_set_elem_t *next = elem->next;
    if (next != NULL) {
        return next->v;
    }

    next = __opa_set_next_bucket(set, i+1);
    if (next != NULL) {
            return next->v;
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

    long long i;

    if (opa_number_try_int(num, &i) != 0)
    {
        return NULL;
    }

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
        case OPA_SET:
            return opa_value_iter_set(opa_cast_set(node), prev);
        }
    }

    return NULL;
}

size_t opa_value_length_object(opa_object_t *obj)
{
    return obj->len;
}

size_t opa_value_length_set(opa_set_t *set)
{
    return set->len;
}

size_t opa_value_length_array(opa_array_t *arr)
{
    return arr->len;
}

size_t opa_value_length_string(opa_string_t *str)
{
    return str->len;
}

OPA_INTERNAL
size_t opa_value_length(opa_value *node)
{
    switch (opa_value_type(node))
    {
    case OPA_ARRAY:
        return opa_value_length_array(opa_cast_array(node));
    case OPA_OBJECT:
        return opa_value_length_object(opa_cast_object(node));
    case OPA_SET:
        return opa_value_length_set(opa_cast_set(node));
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
    long long la, lb;

    if (opa_number_try_int(a, &la) == 0 && opa_number_try_int(b, &lb) == 0)
    {
        if (la < lb)
        {
            return -1;
        }
        else if (la > lb)
        {
            return 1;
        }
        return 0;
    }

    mpd_t *ba = opa_number_to_bf(&a->hdr);
    mpd_t *bb = opa_number_to_bf(&b->hdr);

    uint32_t status = 0;
    int c = mpd_qcmp(ba, bb, &status);
    if (status)
    {
        opa_abort("opa_value_compare_number");
    }

    mpd_del(ba);
    mpd_del(bb);

    return c;
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

int opa_value_compare_set(opa_set_t *a, opa_set_t *b)
{
    opa_array_t *va = __opa_set_values(a);
    opa_array_t *vb = __opa_set_values(b);

    for (size_t i = 0; i < va->len && i < vb->len; i++)
    {
        int cmp = opa_value_compare(opa_value_get_array_native(va, i), opa_value_get_array_native(vb, i));

        if (cmp != 0)
        {
            return cmp;
        }
    }

    if (va->len < vb->len) {
        return -1;
    } else if (va->len > vb->len) {
        return 1;
    }

    return 0;
}

OPA_INTERNAL
int opa_value_compare(opa_value *a, opa_value *b)
{
    if (a == b)
    {
        return 0;
    }
    if (b == NULL)
    {
        return 1;
    }
    if (a == NULL)
    {
        return -1;
    }
    if (opa_value_type(a) < opa_value_type(b))
    {
        return -1;
    }
    if (opa_value_type(b) < opa_value_type(a))
    {
        return 1;
    }

    switch (opa_value_type(a))
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
    case OPA_SET:
    {
        opa_set_t *a1 = opa_cast_set(a);
        opa_set_t *b1 = opa_cast_set(b);
        return opa_value_compare_set(a1, b1);
    }
    default:
        opa_abort("illegal value");
        return 0;
    }
}

#define FNV32_INIT ((size_t)0x811c9dc5)

static size_t
fnv1a32(size_t hash, const void *input, size_t len)
{
    const unsigned char *data = input;
    const unsigned char *end = data + len;

    for (; data != end; ++data)
    {
        hash += (hash<<1) + (hash<<4) + (hash<<7) + (hash<<8) + (hash<<24); // *= 0x01000193
        hash ^= *data;
    }

    return hash;
}

size_t opa_boolean_hash(opa_boolean_t *b) {
    return b->v ? 0 : 1;
}

size_t opa_number_hash(opa_number_t *n) {
    double d = opa_number_as_float(n);
    return fnv1a32(FNV32_INIT, &d, sizeof(d));
}

size_t opa_string_hash(opa_string_t *s) {
    return fnv1a32(FNV32_INIT, s->v, s->len);
}

size_t opa_array_hash(opa_array_t *a) {
    size_t len = opa_value_length_array(a);
    size_t hash = 0;

    for (long long i = 0; i < len; i++)
    {
        hash += opa_value_hash(opa_value_get_array_native(a, i));
    }

    return hash;
}

size_t opa_object_hash(opa_object_t *o) {
    size_t hash = 0;

    for (int i = 0; i < o->n; i++)
    {
        opa_object_elem_t *elem = o->buckets[i];

        while (elem != NULL)
        {
            hash += opa_value_hash(elem->k);
            hash += opa_value_hash(elem->v);
            elem = elem->next;
        }
    }

    return hash;
}

size_t opa_set_hash(opa_set_t *o) {
    size_t hash = 0;

    for (int i = 0; i < o->n; i++)
    {
        opa_set_elem_t *elem = o->buckets[i];

        while (elem != NULL)
        {
            hash += opa_value_hash(elem->v);
            elem = elem->next;
        }
    }

    return hash;
}

size_t opa_value_hash(opa_value *node) {
    switch (opa_value_type(node))
    {
    case OPA_NULL:
        return 0;
    case OPA_BOOLEAN:
        return opa_boolean_hash(opa_cast_boolean(node));
    case OPA_NUMBER:
        return opa_number_hash(opa_cast_number(node));
    case OPA_STRING:
        return opa_string_hash(opa_cast_string(node));
    case OPA_ARRAY:
        return opa_array_hash(opa_cast_array(node));
    case OPA_OBJECT:
        return opa_object_hash(opa_cast_object(node));
    case OPA_SET:
        return opa_set_hash(opa_cast_set(node));
    }

    return 0;
}

OPA_INTERNAL
void opa_value_free(opa_value *node)
{
    switch (node->type) // bypass opa_value_type: don't free OPA_STRING_INTERNED
    {
    case OPA_NULL:
        opa_free(node);
        return;
    case OPA_BOOLEAN:
        opa_free(opa_cast_boolean(node));
        return;
    case OPA_NUMBER:
        opa_number_free(opa_cast_number(node));
        return;
    case OPA_STRING:
        opa_string_free(opa_cast_string(node));
        return;
    case OPA_ARRAY:
        opa_array_free(opa_cast_array(node));
        return;
    case OPA_OBJECT:
        opa_object_free(opa_cast_object(node));
        return;
    case OPA_SET:
        opa_set_free(opa_cast_set(node));
        return;
    }
}

OPA_INTERNAL
opa_value *opa_value_merge(opa_value *a, opa_value *b)
{
    if (a == NULL)
    {
        return b;
    }
    if (opa_value_type(a) != OPA_OBJECT || opa_value_type(b) != OPA_OBJECT)
    {
        return a;
    }

    opa_object_t *obj = opa_cast_object(a);
    opa_object_t *result = opa_cast_object(opa_object());

    for (int i = 0; i < obj->n; i++)
    {
        opa_object_elem_t *elem = obj->buckets[i];

        while (elem != NULL)
        {
            opa_value *other = opa_value_get(b, elem->k);

            if (other == NULL)
            {
                opa_object_insert(result, elem->k, elem->v);
            }
            else
            {
                opa_value *merged = opa_value_merge(elem->v, other);

                if (merged == NULL)
                {
                    return NULL;
                }

                opa_object_insert(result, elem->k, merged);
            }

            elem = elem->next;
        }
    }

    obj = opa_cast_object(b);
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

    return &result->hdr;
}

opa_value *opa_value_shallow_copy_boolean(opa_boolean_t *b)
{
    return opa_boolean(b->v);
}

opa_value *opa_value_shallow_copy_number(opa_number_t *n)
{
    switch (n->repr)
    {
    case OPA_NUMBER_REPR_REF:
        return opa_number_ref(n->v.ref.s, n->v.ref.len);
    case OPA_NUMBER_REPR_INT:
        return opa_number_int(n->v.i);
    default:
        opa_abort("opa_value_shallow_copy_number: illegal repr");
        return NULL;
    }
}

opa_value *opa_value_shallow_copy_string(opa_string_t *s)
{
    return opa_string(s->v, s->len);
}

opa_value *opa_value_shallow_copy_array(opa_array_t *a)
{
    opa_array_elem_t *cpy = (opa_array_elem_t *)opa_malloc(sizeof(opa_array_elem_t) * a->cap);

    for (size_t idx = 0; idx < a->cap; idx++)
    {
        cpy[idx] = a->elems[idx];
    }

    return opa_array_with_elems(cpy, a->len, a->cap);
}

opa_value *opa_value_shallow_copy_object(opa_object_t *o)
{
    opa_value *node = &o->hdr;
    opa_object_t *cpy = opa_cast_object(__opa_object_with_buckets(o->n));
    opa_value *prev = NULL;
    opa_value *curr = NULL;

    while ((curr = opa_value_iter(node, prev)) != NULL)
    {
        opa_value *v = opa_value_get(node, curr);
        opa_object_insert(cpy, curr, v);
        prev = curr;
    }

    return &cpy->hdr;
}

opa_value *opa_value_shallow_copy_set(opa_set_t *s)
{
    opa_value *node = &s->hdr;
    opa_set_t *cpy = opa_cast_set(__opa_set_with_buckets(s->n));
    opa_value *prev = NULL;
    opa_value *curr = NULL;

    while ((curr = opa_value_iter(node, prev)) != NULL)
    {
        opa_set_add(cpy, curr);
        prev = curr;
    }

    return &cpy->hdr;
}

opa_value *opa_value_shallow_copy(opa_value *node)
{
    switch (node->type) // bypass opa_value_type: pass OPA_STRING_INTERNED along as-is
    {
    case OPA_NULL:
        return node;
    case OPA_BOOLEAN:
        return opa_value_shallow_copy_boolean(opa_cast_boolean(node));
    case OPA_NUMBER:
        return opa_value_shallow_copy_number(opa_cast_number(node));
    case OPA_STRING:
        return opa_value_shallow_copy_string(opa_cast_string(node));
    case OPA_ARRAY:
        return opa_value_shallow_copy_array(opa_cast_array(node));
    case OPA_OBJECT:
        return opa_value_shallow_copy_object(opa_cast_object(node));
    case OPA_SET:
        return opa_value_shallow_copy_set(opa_cast_set(node));
    case OPA_STRING_INTERNED:
    case OPA_BOOLEAN_INTERNED:
        return node;
    }

    return NULL;
}

static opa_value *__opa_tuple(opa_value *a, opa_value *b)
{
    opa_value *ret = opa_array_with_cap(2);
    opa_array_t *tuple = opa_cast_array(ret);
    opa_array_append(tuple, a);
    opa_array_append(tuple, b);
    return ret;
}

static void __opa_value_transitive_closure(opa_array_t *result, opa_array_t *path, opa_value *node)
{
    opa_array_append(result, __opa_tuple(&path->hdr, node));
    opa_value *prev = NULL;
    opa_value *curr = NULL;

    while ((curr = opa_value_iter(node, prev)) != NULL)
    {
        opa_array_t *cpy = opa_cast_array(opa_value_shallow_copy_array(path));
        opa_array_append(cpy, curr);
        opa_value *child = opa_value_get(node, curr);
        __opa_value_transitive_closure(result, cpy, child);
        prev = curr;
    }
}

OPA_BUILTIN
opa_value *opa_value_transitive_closure(opa_value *v)
{
    opa_array_t *result = opa_cast_array(opa_array());
    opa_array_t *path = opa_cast_array(opa_array());
    __opa_value_transitive_closure(result, path, v);
    return &result->hdr;
}


OPA_INTERNAL
opa_value *opa_null()
{
    opa_value *ret = (opa_value *)opa_malloc(sizeof(opa_value));
    ret->type = OPA_NULL;
    return ret;
}

opa_value *opa_boolean_allocated(bool v)
{
    opa_boolean_t *ret = (opa_boolean_t *)opa_malloc(sizeof(opa_boolean_t));
    ret->hdr.type = OPA_BOOLEAN;
    ret->v = v;
    return &ret->hdr;
}

opa_value *opa_boolean(bool v)
{
    return opa_boolean_allocated(v);
}

OPA_INTERNAL
opa_value *opa_number_size(size_t v)
{
    opa_number_t *ret = (opa_number_t *)opa_malloc(sizeof(opa_number_t));
    ret->hdr.type = OPA_NUMBER;
    ret->repr = OPA_NUMBER_REPR_INT;
    ret->v.i = (long long)v;
    return &ret->hdr;
}

OPA_INTERNAL
opa_value *opa_number_int(long long v)
{
    opa_number_t *ret = (opa_number_t *)opa_malloc(sizeof(opa_number_t));
    ret->hdr.type = OPA_NUMBER;
    ret->repr = OPA_NUMBER_REPR_INT;
    ret->v.i = v;
    return &ret->hdr;
}

OPA_INTERNAL
opa_value *opa_number_ref(const char *s, size_t len)
{
    opa_number_t *ret = (opa_number_t *)opa_malloc(sizeof(opa_number_t));
    ret->hdr.type = OPA_NUMBER;
    ret->repr = OPA_NUMBER_REPR_REF;
    ret->v.ref.s = s;
    ret->v.ref.len = len;
    ret->v.ref.free = 0;
    return &ret->hdr;
}

opa_value *opa_number_ref_allocated(const char *s, size_t len)
{
    opa_number_t *ret = (opa_number_t *)opa_malloc(sizeof(opa_number_t));
    ret->hdr.type = OPA_NUMBER;
    ret->repr = OPA_NUMBER_REPR_REF;
    ret->v.ref.s = s;
    ret->v.ref.len = len;
    ret->v.ref.free = 1;
    return &ret->hdr;
}

void opa_number_init_int(opa_number_t *n, long long v)
{
    n->hdr.type = OPA_NUMBER;
    n->repr = OPA_NUMBER_REPR_INT;
    n->v.i = v;
}

void opa_number_free(opa_number_t *n)
{
    if (n->repr == OPA_NUMBER_REPR_REF)
    {
        if (n->v.ref.free)
        {
            opa_free((void *)n->v.ref.s);
        }
    }

    opa_free(n);
}

int opa_number_try_int(opa_number_t *n, long long *i)
{
    switch (n->repr)
    {
    case OPA_NUMBER_REPR_INT:
        *i = n->v.i;
        return 0;
    case OPA_NUMBER_REPR_REF:
        return opa_atoi64(n->v.ref.s, n->v.ref.len, i);
    default:
        opa_abort("opa_number_try_int: illegal repr");
        return -1;
    }
}

double opa_number_as_float(opa_number_t *n)
{
    switch (n->repr)
    {
    case OPA_NUMBER_REPR_INT:
        return (double)n->v.i;
    case OPA_NUMBER_REPR_REF:
    {
        double d;
        int rc = opa_atof64(n->v.ref.s, n->v.ref.len, &d);
        if (rc != 0)
        {
            opa_abort("opa_number_as_float: illegal ref");
        }
        return d;
    }
    default:
        opa_abort("opa_number_as_float: illegal repr");
        return 0.0;
    }
}

opa_value *opa_string(const char *v, size_t len)
{
    opa_string_t *ret = (opa_string_t *)opa_malloc(sizeof(opa_string_t));
    ret->hdr.type = OPA_STRING;
    ret->free = 0;
    ret->len = len;
    ret->v = v;
    return &ret->hdr;
}

OPA_INTERNAL
opa_value *opa_string_terminated(const char *v)
{
    opa_string_t *ret = (opa_string_t *)opa_malloc(sizeof(opa_string_t));
    ret->hdr.type = OPA_STRING;
    ret->free = 0;
    ret->len = opa_strlen(v);
    ret->v = v;
    return &ret->hdr;
}

opa_value *opa_string_allocated(const char *v, size_t len)
{
    opa_string_t *ret = (opa_string_t *)opa_malloc(sizeof(opa_string_t));
    ret->hdr.type = OPA_STRING;
    ret->free = 1;
    ret->len = len;
    ret->v = v;
    return &ret->hdr;
}

void opa_string_free(opa_string_t *s)
{
    if (s->free)
    {
        opa_free((void *)s->v);
    }

    opa_free(s);
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

    if (arr->elems != NULL)
    {
        opa_free(arr->elems);
    }
    arr->elems = elems;
}

opa_value *opa_array()
{
    return opa_array_with_cap(0);
}

OPA_INTERNAL
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

opa_value *opa_array_with_elems(opa_array_elem_t *elems, size_t len, size_t cap)
{
    opa_array_t *ret = (opa_array_t *)opa_malloc(sizeof(opa_array_t));

    ret->hdr.type = OPA_ARRAY;
    ret->len = len;
    ret->cap = cap;
    ret->elems = elems;

    return &ret->hdr;
}

static opa_value *__opa_object_with_buckets(size_t buckets)
{
    opa_object_t *ret = (opa_object_t *)opa_malloc(sizeof(opa_object_t));
    ret->hdr.type = OPA_OBJECT;
    ret->buckets = (opa_object_elem_t **)opa_malloc(sizeof(opa_object_elem_t *) * buckets);
    ret->n = buckets;
    ret->len = 0;

    for (size_t i = 0; i < buckets; i++) {
        ret->buckets[i] = NULL;
    }

    return &ret->hdr;
}

opa_value *opa_object()
{
    return __opa_object_with_buckets(OPA_OBJECT_MIN_BUCKETS);
}

static opa_value *__opa_set_with_buckets(size_t buckets)
{
    opa_set_t *ret = (opa_set_t *)opa_malloc(sizeof(opa_set_t));
    ret->hdr.type = OPA_SET;
    ret->buckets = (opa_set_elem_t **)opa_malloc(sizeof(opa_set_elem_t *) * buckets);
    ret->n = buckets;
    ret->len = 0;

    for (size_t i = 0; i < buckets; i++) {
        ret->buckets[i] = NULL;
    }

    return &ret->hdr;
}

OPA_INTERNAL
opa_value *opa_set()
{
    return __opa_set_with_buckets(OPA_SET_MIN_BUCKETS);
}

opa_value *opa_set_with_cap(size_t n)
{
    size_t buckets = OPA_SET_MIN_BUCKETS;

    while (n > (buckets * OPA_SET_LOAD_FACTOR))
    {
        buckets *= 2;
    }

    return __opa_set_with_buckets(buckets);
}

OPA_INTERNAL
void opa_value_number_set_int(opa_value *v, long long i)
{
	opa_number_t *ret = opa_cast_number(v);
	ret->repr = OPA_NUMBER_REPR_INT;
	ret->v.i = i;
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

OPA_INTERNAL
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
        opa_value *elem = arr->elems[i].v;
        size_t j = i - 1;

        while (j >= 0 && cmp_fn(arr->elems[j].v, elem) > 0)
        {
            arr->elems[j + 1].v = arr->elems[j].v;
            j = j - 1;
        }

        arr->elems[j + 1].v = elem;
    }
}

void __opa_object_buckets_free(opa_object_t *obj)
{
    for (int i = 0; i < obj->n; i++)
    {
        opa_object_elem_t *prev = NULL;

        for (opa_object_elem_t *curr = obj->buckets[i]; curr != NULL; curr = curr->next)
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

    opa_free(obj->buckets);
}

void opa_object_free(opa_object_t *obj)
{
    __opa_object_buckets_free(obj);
    opa_free(obj);
}

opa_array_t *opa_object_keys(opa_object_t *obj)
{
    opa_array_t *keys = opa_cast_array(opa_array_with_cap(opa_value_length_object(obj)));

    for (int i = 0; i < obj->n; i++)
    {
        opa_object_elem_t *elem = obj->buckets[i];

        while (elem != NULL)
        {
            opa_array_append(keys, elem->k);
            elem = elem->next;
        }
    }

    opa_array_sort(keys, opa_value_compare);
    return keys;
}

opa_object_elem_t *__opa_object_elem_alloc(opa_value *k, opa_value *v)
{
    opa_object_elem_t *elem = (opa_object_elem_t *)opa_malloc(sizeof(opa_object_elem_t));
    elem->next = NULL;
    elem->k = k;
    elem->v = v;
    return elem;
}

void __opa_object_grow(opa_object_t *obj, size_t n) {
    if (n <= (obj->n * OPA_OBJECT_LOAD_FACTOR))
    {
        return;
    }

    opa_object_t *dst = opa_cast_object(__opa_object_with_buckets(obj->n * 2));

    for (int i = 0; i < obj->n; i++)
    {
        opa_object_elem_t *elem = obj->buckets[i];

        while (elem != NULL)
        {
            opa_object_elem_t *next = elem->next;
            __opa_object_insert_elem(dst, elem, opa_value_hash(elem->k));
            elem = next;
        }
    }

    opa_free(obj->buckets);
    obj->buckets = dst->buckets;
    obj->n = dst->n;
    opa_free(dst);
}

OPA_INTERNAL
void opa_object_insert(opa_object_t *obj, opa_value *k, opa_value *v)
{
    size_t hash = opa_value_hash(k);

    for (opa_object_elem_t *curr = obj->buckets[hash % obj->n]; curr != NULL; curr = curr->next)
    {
        if (opa_value_compare(curr->k, k) == 0)
        {
            curr->v = v;
            return;
        }
    }

    __opa_object_grow(obj, obj->len+1);
    __opa_object_insert_elem(obj, __opa_object_elem_alloc(k, v), hash);
}

static void __opa_object_insert_elem(opa_object_t *obj, opa_object_elem_t *new, size_t hash)
{
    size_t i = hash % obj->n;
    opa_object_elem_t **prev = &obj->buckets[i];
    opa_object_elem_t *curr = obj->buckets[i];

    while (1) {
        if (curr == NULL || opa_value_compare(new->k, curr->k) < 0) {
            *prev = new;
            new->next = curr;
            break;
        }

        prev = &curr->next;
        curr = curr->next;
    }

    obj->len++;
}

void opa_object_remove(opa_object_t *obj, opa_value *k)
{
    size_t hash = opa_value_hash(k);

    size_t i = hash % obj->n;
    opa_object_elem_t **prev = &obj->buckets[i];
    opa_object_elem_t *curr = obj->buckets[i];
    while (curr != NULL)
    {
        if (opa_value_compare(curr->k, k) == 0)
        {
            *prev = curr->next;
            obj->len--;

            opa_value_free(curr->k);
            opa_value_free(curr->v);
            opa_free(curr);

            // TODO: Consider shrinking the object size. For now it will remain
            // with its current size.

            return;
        }
        prev = &curr->next;
        curr = curr->next;
    }
    return;  // Key wasn't found, consider it deleted.
}

opa_object_elem_t *opa_object_get(opa_object_t *obj, opa_value *key)
{
    size_t hash = opa_value_hash(key) % obj->n;

    for (opa_object_elem_t *curr = obj->buckets[hash]; curr != NULL; curr = curr->next)
    {
        if (opa_value_compare(curr->k, key) == 0)
        {
            return curr;
        }
    }

    return NULL;
}

void __opa_set_buckets_free(opa_set_t *set)
{
    for (int i = 0; i < set->n; i++)
    {
        opa_set_elem_t *prev = NULL;

        for (opa_set_elem_t *curr = set->buckets[i]; curr != NULL; curr = curr->next)
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

    opa_free(set->buckets);
}

void opa_set_free(opa_set_t *set)
{
    __opa_set_buckets_free(set);
    opa_free(set);
}

opa_array_t *__opa_set_values(opa_set_t *set)
{
    opa_array_t *values = opa_cast_array(opa_array_with_cap(opa_value_length_set(set)));

    for (int i = 0; i < set->n; i++)
    {
        opa_set_elem_t *elem = set->buckets[i];

        while (elem != NULL)
        {
            opa_array_append(values, elem->v);
            elem = elem->next;
        }
    }

    opa_array_sort(values, opa_value_compare);
    return values;
}

opa_set_elem_t *__opa_set_elem_alloc(opa_value *v)
{
    opa_set_elem_t *elem = (opa_set_elem_t *)opa_malloc(sizeof(opa_set_elem_t));
    elem->next = NULL;
    elem->v = v;
    return elem;
}

void __opa_set_grow(opa_set_t *set, size_t n) {
    if (n <= (set->n * OPA_SET_LOAD_FACTOR))
    {
        return;
    }

    opa_set_t *dst = opa_cast_set(__opa_set_with_buckets(set->n * 2));

    for (int i = 0; i < set->n; i++)
    {
        opa_set_elem_t *elem = set->buckets[i];

        while (elem != NULL)
        {
            opa_set_elem_t *next = elem->next;
            __opa_set_add_elem(dst, elem, opa_value_hash(elem->v));
            elem = next;
        }
    }

    opa_free(set->buckets);
    set->buckets = dst->buckets;
    set->n = dst->n;
    opa_free(dst);
}

OPA_INTERNAL
void opa_set_add(opa_set_t *set, opa_value *v)
{
    size_t hash = opa_value_hash(v);

    for (opa_set_elem_t *curr = set->buckets[hash % set->n]; curr != NULL; curr = curr->next)
    {
        if (opa_value_compare(curr->v, v) == 0)
        {
            return;
        }
    }

    __opa_set_grow(set, set->len+1);
    __opa_set_add_elem(set, __opa_set_elem_alloc(v), hash);
}

static void __opa_set_add_elem(opa_set_t *set, opa_set_elem_t *new, size_t hash)
{
    size_t i = hash % set->n;
    opa_set_elem_t **prev = &set->buckets[i];
    opa_set_elem_t *curr = set->buckets[i];

    while (1) {
        if (curr == NULL || opa_value_compare(new->v, curr->v) < 0) {
            *prev = new;
            new->next = curr;
            break;
        }

        prev = &curr->next;
        curr = curr->next;
    }

    set->len++;
}

opa_set_elem_t *opa_set_get(opa_set_t *set, opa_value *v)
{
    size_t hash = opa_value_hash(v) % set->n;

    for (opa_set_elem_t *curr = set->buckets[hash]; curr != NULL; curr = curr->next)
    {
        if (opa_value_compare(curr->v, v) == 0)
        {
            return curr;
        }
    }

    return NULL;
}

// Validate that a path is non-null, an array value type,
// has a length >0, and only contains strings.
// Returns the size of the path, or -1 for an invalid path
int _validate_json_path(opa_value *path)
{
    if (path == NULL || opa_value_type(path) != OPA_ARRAY)
    {
        return -1;
    }

    int path_len = opa_value_length(path);
    if (path_len == 0)
    {
        return -1;
    }

    for (int i = 0; i < path_len-1; i++)
    {
        opa_value *v = opa_value_get_array_native(opa_cast_array(path), i);
        if (opa_value_type(v) != OPA_STRING)
        {
            return -1;
        }
    }

    return path_len;
}

// For the given `data` value set the provided value `v` at
// the specified `path`. Requires objects for containers,
// any portion of the path that is missing will be created.
// The `path` must be an `opa_array_t` with at least one
// element.
//
// Replaced objects will be freed.
WASM_EXPORT(opa_value_add_path)
opa_errc opa_value_add_path(opa_value *data, opa_value *path, opa_value *v)
{
    int path_len = _validate_json_path(path);

    if (path_len < 1)
    {
        return OPA_ERR_INVALID_PATH;
    }

    // Follow the path, creating objects as needed.
    opa_array_t *p = opa_cast_array(path);
    opa_value *curr = data;

    for (int i = 0; i < path_len-1; i++)
    {
        opa_value *k = opa_value_get_array_native(p, i);

        opa_value *next = opa_value_get(curr, k);

        if (next == NULL)
        {
            switch (curr->type)
            {
                case OPA_OBJECT:
                    next = opa_object();
                    opa_object_insert(opa_cast_object(curr), k, next);
                    break;
                default:
                    return OPA_ERR_INVALID_TYPE;
            }
        }

        curr = next;
    }

    opa_value *k = opa_value_get_array_native(p, path_len-1);

    opa_value *old = opa_value_get(curr, k);

    switch (curr->type)
    {
        case OPA_OBJECT:
            opa_object_insert(opa_cast_object(curr), k, v);
            break;
        default:
            return OPA_ERR_INVALID_TYPE;
    }

    if (old != NULL)
    {
        opa_value_free(old);
    }

    return OPA_ERR_OK;
}

// For the given `data` object delete the entry specified by `path`.
// The `path` must be an `opa_array_t` with at least one
// element.
//
// Deleted values will be freed.
WASM_EXPORT(opa_value_remove_path)
opa_errc opa_value_remove_path(opa_value *data, opa_value *path)
{
    int path_len = _validate_json_path(path);

    if (path_len < 1)
    {
        return OPA_ERR_INVALID_PATH;
    }

    // Follow the path into data
    opa_array_t *p = opa_cast_array(path);
    opa_value *curr = data;

    for (int i = 0; i < path_len-1; i++)
    {
        opa_value *k = opa_value_get_array_native(p, i);

        opa_value *next = opa_value_get(curr, k);

        if (next == NULL)
        {
            // We were unable to follow the full
            // path, consider the target deleted
            return OPA_ERR_OK;
        }

       curr = next;
    }

    opa_object_remove(opa_cast_object(curr), opa_value_get_array_native(p, path_len-1));

    return OPA_ERR_OK;
}

// Lookup path in the passed mapping object. Returns 0 if it can't
// be found, or of there's no function index leaf when we've run out
// of path pieces.
int opa_lookup(opa_value *mapping, opa_value *path) {
    if (path == NULL || opa_value_type(path) != OPA_ARRAY)
    {
        return 0;
    }

    int path_len = opa_value_length(path);
    if (path_len == 0)
    {
        return 0;
    }

    opa_value *curr = mapping;

    for (opa_value *idx = opa_value_iter(path, NULL); idx != NULL; idx = opa_value_iter(path, idx))
    {
        opa_value *key = opa_value_get(path, idx);
        opa_value *next = opa_value_get(curr, key);
        if (next == NULL)
        {
            return 0;
        }
        curr = next;
    }
    if (curr->type == OPA_NUMBER) {
        long long i;
        if (opa_number_try_int(opa_cast_number(curr), &i) == 0) {
            return i;
        }
    }
    return 0;
}

// global variable used for storing the parsed mapping JSON
static opa_value *mapping;

// Called from the WASM-generated '_initialize' function with the
// address of the mapping string and its length. Parses the JSON
// string it expects, sets the *mapping variable accordingly.
OPA_INTERNAL
void opa_mapping_init(const char *s, const int l) {
    if (mapping == NULL) {
        mapping = opa_json_parse(s, l);
    }
}

// Lookup mapped function index from global mapping (initialized by
// opa_mapping_init).
OPA_INTERNAL
int opa_mapping_lookup(opa_value *path) {
    return opa_lookup(mapping, path);
}
