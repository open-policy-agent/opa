#include "memoize.h"
#include "malloc.h"
#include "std.h"

struct memoize {
    struct memoize  *prev;
    opa_object_t    *table;
};

static struct memoize *m = NULL;

struct memoize *opa_memoize_alloc(struct memoize *prev)
{
    struct memoize *result = (struct memoize *)opa_malloc(sizeof(struct memoize));
    result->prev = prev;
    result->table = opa_cast_object(opa_object());
    return result;
}

OPA_INTERNAL
void opa_memoize_init(void)
{
    m = opa_memoize_alloc(NULL);
}

OPA_INTERNAL
void opa_memoize_push(void)
{
    m = opa_memoize_alloc(m);
}

OPA_INTERNAL
void opa_memoize_pop(void)
{
    // NOTE(tsandall): free() is not called because we assume the heap will be
    // reset on the next eval() call.
    m = m->prev;
}

OPA_INTERNAL
void opa_memoize_insert(int32_t index, opa_value *value)
{
    // NOTE(tsandall): allocating a number is suboptimal but worst-case is ~1 per
    // planned rule so overhead should be minimal compared to the rest of evaluation.
    opa_value *key = opa_number_int(index);
    opa_object_insert(m->table, key, value);
}

OPA_INTERNAL
opa_value *opa_memoize_get(int32_t index)
{
    opa_number_t key;
    opa_number_init_int(&key, index);
    opa_object_elem_t *elem = opa_object_get(m->table, &key.hdr);

    if (elem == NULL)
    {
        return NULL;
    }

    return elem->v;
}
