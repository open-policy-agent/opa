#include <string.h>

#define WASM_PAGE_SIZE (65536)

static int initialized;
static unsigned int heap_ptr;
static unsigned int heap_top;
extern unsigned char __heap_base; // set by lld


unsigned int opa_heap_ptr_get(void)
{
    return heap_ptr;
}

unsigned int opa_heap_top_get(void)
{
    return heap_top;
}

void opa_heap_ptr_set(unsigned int ptr)
{
    heap_ptr = ptr;
}

void opa_heap_top_set(unsigned int top)
{
    heap_top = top;
}

void *opa_malloc(size_t size)
{
    if (!initialized)
    {
        heap_ptr = (unsigned int)&__heap_base;
        heap_top = __builtin_wasm_grow_memory(0) * WASM_PAGE_SIZE;
        initialized = 1;
    }

    unsigned int ptr = heap_ptr;
    heap_ptr += size;

    if (heap_ptr >= heap_top)
    {
        unsigned int pages = (size / WASM_PAGE_SIZE) + 1;
        __builtin_wasm_grow_memory(pages);
        heap_top += (pages * WASM_PAGE_SIZE);
    }

    return (void *)ptr;
}

void opa_free(void *ptr)
{
}
