#include "std.h"

#define WASM_PAGE_SIZE (65536)

static unsigned int heap;

static unsigned int grow_heap(size_t size)
{
    size_t pages = (size / WASM_PAGE_SIZE) + 1;
    return __builtin_wasm_grow_memory(pages) * WASM_PAGE_SIZE;
}

void *opa_malloc(size_t size)
{
    return (void *)grow_heap(size);
}

void opa_free(void *ptr)
{
}