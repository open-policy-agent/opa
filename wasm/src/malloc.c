#include <string.h>
#include "std.h"

#define WASM_PAGE_SIZE (65536)

static int initialized;
static unsigned int heap_ptr;
static unsigned int heap_top;
extern unsigned char __heap_base; // set by lld

struct heap_block {
    size_t size;
    struct heap_block *prev; // unset if block allocated
    struct heap_block *next; // unset if block allocated
    unsigned char data[0];
};

// all the free blocks, ordered per their memory address.
static struct heap_block heap_free_start;
static struct heap_block heap_free_end;

#define HEAP_CHECK()
//#define HEAP_CHECK() heap_check(__FUNCTION__)

static void init_free()
{
    heap_free_start.size = 0;
    heap_free_start.next = &heap_free_end;
    heap_free_end.size = 0;
    heap_free_end.prev = &heap_free_start;
}

static void heap_check(const char *name)
{
    for (struct heap_block *b = heap_free_start.next, *prev = &heap_free_start; b != &heap_free_end; prev = b, b = b->next) {
        if (prev == NULL || b == NULL || b->prev != prev) {
            opa_abort(name);
        }
    }

    for (struct heap_block *b = heap_free_end.prev, *next = &heap_free_end; b != &heap_free_start; next = b, b = b->prev) {
        if (next == NULL || b == NULL || b->next != next) {
            opa_abort(name);
        }
    }
}

// try removing the last block from the free list and adjusting the heap pointer accordingly.
static void compact_free()
{
    struct heap_block *last = heap_free_end.prev;

    if (last == &heap_free_start)
    {
        return;
    }

    if (((void *)(&last->data[0]) + last->size) == (void *)heap_ptr)
    {
        heap_ptr -= sizeof(struct heap_block) + last->size;
        last->prev->next = &heap_free_end;
        heap_free_end.prev = last->prev;
    }

    HEAP_CHECK();
}

static void init(void)
{
    if (!initialized)
    {
        heap_ptr = (unsigned int)&__heap_base;
        heap_top = __builtin_wasm_grow_memory(0) * WASM_PAGE_SIZE;
        init_free();
        initialized = 1;
    }
}

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
    init_free();
}

void opa_heap_top_set(unsigned int top)
{
    heap_top = top;
    init_free();
}

void *opa_malloc(size_t size)
{
    init();
    HEAP_CHECK();

    // Look for the first free block that is large enough. Split the found block if necessary.

    for (struct heap_block *b = heap_free_start.next; b != &heap_free_end; b = b->next)
    {
        if (b->size > (sizeof(struct heap_block) + size))
        {
            struct heap_block *remaining = (void *)(&b->data[0]) + size;
            remaining->size = b->size - (sizeof(struct heap_block) + size);
            remaining->prev = b->prev;
            remaining->next = b->next;
            remaining->prev->next = remaining;
            remaining->next->prev = remaining;

            b->size = size;
            b->prev = NULL;
            b->next = NULL;

            HEAP_CHECK();

            return b->data;
        } else if (b->size == size)
        {
            b->prev->next = b->next;
            b->next->prev = b->prev;
            b->prev = NULL;
            b->next = NULL;

            HEAP_CHECK();

            return b->data;
        }
    }

    // Allocate a new block.

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
    b->prev = NULL;
    b->next = NULL;

    return b->data;
}

void opa_free(void *ptr)
{
    if (ptr == NULL)
    {
        opa_abort("opa_free: null pointer");
    }

    struct heap_block *block = ptr - sizeof(struct heap_block);
    struct heap_block *prev = &heap_free_start;

    if (block->prev != NULL || block->next != NULL)
    {
        opa_abort("opa_free: double free");
    }

    HEAP_CHECK();

    // Find the free block available just before this block and try to
    // defragment, by trying to merge with this block with the found
    // block and the one after.

    for (struct heap_block *b = prev->next; b < block && b != &heap_free_end; prev = b, b = b->next);

    struct heap_block *prev_end = (void *)(&prev->data[0]) + prev->size;
    struct heap_block *block_end = (void *)(&block->data[0]) + block->size;

    if (prev_end == block)
    {
        prev->size += sizeof(struct heap_block) + block->size;
        compact_free();
        return;
    }

    if (block_end == prev->next)
    {
        struct heap_block *next = prev->next;
        block->prev = prev;
        block->next = next->next;
        block->size += sizeof(struct heap_block) + next->size;

        prev->next = block;
        block->next->prev = block;
        compact_free();
        return;
    }

    // List the block as free.

    block->prev = prev;
    block->next = prev->next;
    prev->next = block;
    block->next->prev = block;
    compact_free();
}

void *opa_realloc(void *ptr, size_t size)
{
    struct heap_block *block = ptr - sizeof(struct heap_block);
    void *p = opa_malloc(size);

    memcpy(p, ptr, block->size < size ? block->size : size);
    opa_free(ptr);
    return p;
}

// Count the number of free blocks. This is for testing only.
size_t opa_heap_free_blocks(void)
{
    init();

    size_t blocks1 = 0, blocks2 = 0;

    for (struct heap_block *b = heap_free_start.next; b != &heap_free_end; b = b->next, blocks1++);
    for (struct heap_block *b = heap_free_end.prev; b != &heap_free_start; b = b->prev, blocks2++);

    if (blocks1 != blocks2)
    {
        opa_abort("opa_malloc: corrupted heap");
    }

    HEAP_CHECK();

    return blocks1;
}
