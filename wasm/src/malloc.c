#include <string.h>

#define WASM_PAGE_SIZE (65536)

static int initialized;
static unsigned int heap_ptr;
static unsigned int heap_top;
extern unsigned char __heap_base; // set by lld

struct heap_block {
    size_t size;
    unsigned char data[0];
};

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
    size_t block_size = sizeof(struct heap_block) + size;
    heap_ptr += block_size;

    if (heap_ptr >= heap_top)
    {
        unsigned int pages = (block_size / WASM_PAGE_SIZE) + 1;
        __builtin_wasm_grow_memory(pages);
        heap_top += (pages * WASM_PAGE_SIZE);
    }

    struct heap_block *b = (void *)ptr;
    b->size = size;
    return b->data;
}

void opa_free(void *ptr)
{
}

void *opa_realloc(void *ptr, size_t size)
{
    struct heap_block *block = ptr - sizeof(struct heap_block);
    void *p = opa_malloc(size);

    memcpy(p, ptr, block->size < size ? block->size : size);
    return p;
}
