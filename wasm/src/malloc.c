#include <string.h>
#include "std.h"
#include "stdlib.h"

#define WASM_PAGE_SIZE (65536)

#define ARRAY_SIZE(ARRAY) (sizeof(ARRAY) / sizeof((ARRAY)[0]))

static int initialized;
static unsigned int heap_ptr;
static unsigned int heap_top;
extern unsigned char __heap_base; // set by lld
static void *builtin_cache[8];

struct heap_block {
    size_t size;
    struct heap_block *prev; // unset if block allocated
    struct heap_block *next; // unset if block allocated
    unsigned char data[0];
};

// free blocks, ordered per their memory address.
struct heap_blocks {
    bool fixed_size;
    size_t size; // if fixed size, this indicates the block size.
                 // if not fixed, this indicates the minimum block size.
    struct heap_block start;
    struct heap_block end;
};

// all the free blocks: fixed size blocks of 4, 8, 16 and 64 bytes and then one free
// list for varying sized blocks of 128 bytes or more.
static struct heap_blocks heap_free[5] = {
    {true, 4},
    {true, 8},
    {true, 16},
    {true, 64},
    {false, 128},
};

#ifdef DEBUG
#define HEAP_CHECK(blocks) heap_check(__FUNCTION__, blocks)
#else
#define HEAP_CHECK(blocks)
#endif

static void init_free()
{
    for (int i = 0; i < ARRAY_SIZE(heap_free); i++) {
        heap_free[i].start = (struct heap_block) { 0, NULL, &heap_free[i].end };
        heap_free[i].end = (struct heap_block) { 0, &heap_free[i].start, NULL };
    }

    for (int i = 0; i < ARRAY_SIZE(builtin_cache); i++)
    {
        builtin_cache[i] = NULL;
    }
}

static void heap_check(const char *name, struct heap_blocks *blocks)
{
    struct heap_block *start = &blocks->start;
    struct heap_block *end = &blocks->end;

    for (struct heap_block *b = start->next, *prev = start; b != end; prev = b, b = b->next) {
        if (prev == NULL || b == NULL || b->prev != prev) {
            opa_abort(name);
        }
    }

    for (struct heap_block *b = end->prev, *next = end; b != start; next = b, b = b->prev) {
        if (next == NULL || b == NULL || b->next != next) {
            opa_abort(name);
        }
    }
}

// try removing the last block(s) from the free list and adjusting the heap pointer accordingly.
static bool compact_free(struct heap_blocks *blocks)
{
    struct heap_block *start = &blocks->start;
    struct heap_block *end = &blocks->end;
    unsigned int old_heap_ptr = heap_ptr;

    while (1)
    {
        struct heap_block *last = end->prev;

        if (last == start)
        {
            break;
        }

        if (((void *)(&last->data[0]) + last->size) != (void *)heap_ptr)
        {
            break;
        }

        heap_ptr -= sizeof(struct heap_block) + last->size;
        last->prev->next = end;
        end->prev = last->prev;
    }

    HEAP_CHECK(blocks);
    return old_heap_ptr != heap_ptr;
}

static void init(void)
{
    if (!initialized)
    {
        heap_ptr = (unsigned int)&__heap_base;
        heap_top = __builtin_wasm_memory_grow(0, 0) * WASM_PAGE_SIZE;
        init_free();
        initialized = 1;
    }
}

static struct heap_block * __opa_malloc_reuse_fixed(struct heap_blocks *blocks);
static struct heap_block * __opa_malloc_reuse_varying(struct heap_blocks *blocks, size_t size);

WASM_EXPORT(opa_heap_ptr_get)
unsigned int opa_heap_ptr_get(void)
{
    return heap_ptr;
}

unsigned int opa_heap_top_get(void)
{
    return heap_top;
}

WASM_EXPORT(opa_heap_ptr_set)
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

// returns the free list applicable for the requested size.
static struct heap_blocks * __opa_blocks(size_t size) {
    for (int i = 0; i < ARRAY_SIZE(heap_free)-1; i++) {
        struct heap_blocks *candidate = &heap_free[i];

        if (size <= candidate->size)
        {
            return candidate;
        }
    }

    return &heap_free[ARRAY_SIZE(heap_free)-1];
}

static void *__opa_malloc_new_allocation(size_t size)
{
    unsigned int ptr = heap_ptr;
    size_t block_size = sizeof(struct heap_block) + size;
    heap_ptr += block_size;

    if (heap_ptr >= heap_top)
    {
        unsigned int pages = (block_size / WASM_PAGE_SIZE) + 1;
        __builtin_wasm_memory_grow(0, pages);
        heap_top += (pages * WASM_PAGE_SIZE);
    }

    struct heap_block *b = (void *)ptr;
    b->size = size;
    b->prev = NULL;
    b->next = NULL;

    return b->data;
}

WASM_EXPORT(opa_malloc)
void *opa_malloc(size_t size)
{
    init();

    // Look for the first free block that is large enough. Split the found block if necessary.

    struct heap_blocks *blocks = __opa_blocks(size);
    HEAP_CHECK(blocks);

    struct heap_block *b = blocks->fixed_size ?
        __opa_malloc_reuse_fixed(blocks) : __opa_malloc_reuse_varying(blocks, size);
    if (b != NULL)
    {
        return b->data;
    }

    // Allocate a new block.

    if (blocks->fixed_size)
    {
        size = blocks->size;
    }

    return __opa_malloc_new_allocation(size);
}

// returns a free block from the list, if available.
static struct heap_block * __opa_malloc_reuse_fixed(struct heap_blocks *blocks)
{
    struct heap_block *end = &blocks->end;
    struct heap_block *b = blocks->start.next;

    if (b != end)
    {
        b->prev->next = b->next;
        b->next->prev = b->prev;
        b->prev = NULL;
        b->next = NULL;

        HEAP_CHECK(blocks);

        return b;
    }

    return NULL;
}

// finds a free block at least of given size, splitting the found block if the remaining block exceeds the minimum size.
static struct heap_block * __opa_malloc_reuse_varying(struct heap_blocks *blocks, size_t size)
{
    struct heap_block *start = &blocks->start;
    struct heap_block *end = &blocks->end;
    size_t min_size = blocks->size;

    for (struct heap_block *b = start->next; b != end; b = b->next)
    {
        if (b->size >= (sizeof(struct heap_block) + min_size + size))
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

            HEAP_CHECK(blocks);

            return b;
        } else if (b->size >= size)
        {
            b->prev->next = b->next;
            b->next->prev = b->prev;
            b->prev = NULL;
            b->next = NULL;

            HEAP_CHECK(blocks);

            return b;
        }
    }
    return NULL;
}

WASM_EXPORT(opa_free)
void opa_free(void *ptr)
{
    struct heap_block *block = ptr - sizeof(struct heap_block);

#ifdef DEBUG
    if (ptr == NULL)
    {
        opa_abort("opa_free: null pointer");
    }

    if (block->prev != NULL || block->next != NULL)
    {
        opa_abort("opa_free: double free");
    }
#endif

    struct heap_blocks *blocks = __opa_blocks(block->size);
    struct heap_block *start = &blocks->start;
    struct heap_block *end = &blocks->end;
    bool fixed_size = blocks->fixed_size;

    HEAP_CHECK(blocks);

    // Find the free block available just before this block and try to
    // defragment, by trying to merge with this block with the found
    // block and the one after.

    struct heap_block *prev = start;
    for (struct heap_block *b = prev->next; b < block && b != end; prev = b, b = b->next);

    if (!fixed_size)
    {
        struct heap_block *prev_end = (void *)(&prev->data[0]) + prev->size;
        struct heap_block *block_end = (void *)(&block->data[0]) + block->size;

        if (prev_end == block)
        {
            prev->size += sizeof(struct heap_block) + block->size;
            compact_free(blocks);
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
            compact_free(blocks);
            return;
        }
    }

    // List the block as free.

    block->prev = prev;
    block->next = prev->next;
    prev->next = block;
    block->next->prev = block;
    compact_free(blocks);
}

void *opa_realloc(void *ptr, size_t size)
{
    struct heap_block *block = ptr - sizeof(struct heap_block);
    void *p = opa_malloc(size);

    memcpy(p, ptr, block->size < size ? block->size : size);
    opa_free(ptr);
    return p;
}

static void **__opa_builtin_cache(size_t i)
{
    if (i >= ARRAY_SIZE(builtin_cache))
    {
        opa_abort("opa_malloc: illegal builtin cache index");
    }

    return &builtin_cache[i];
}

void *opa_builtin_cache_get(size_t i)
{
    return *__opa_builtin_cache(i);
}

void opa_builtin_cache_set(size_t i, void *p)
{
    *__opa_builtin_cache(i) = p;
}

// Compact all the free blocks. This is for testing only.
void opa_heap_compact(void)
{
    for (bool progress = true; progress; )
    {
        progress = false;
        for (int i = 0; i < ARRAY_SIZE(heap_free); i++) {
            progress |= compact_free(&heap_free[i]);
        }
    }
}

// Count the number of free blocks. This is for testing only.
size_t opa_heap_free_blocks(void)
{
    init();

    size_t blocks1 = 0, blocks2 = 0;

    for (int i = 0; i < ARRAY_SIZE(heap_free); i++)
    {
        for (struct heap_block *b = heap_free[i].start.next; b != &heap_free[i].end; b = b->next, blocks1++);
        for (struct heap_block *b = heap_free[i].end.prev; b != &heap_free[i].start; b = b->prev, blocks2++);

        if (blocks1 != blocks2)
        {
            opa_abort("opa_malloc: corrupted heap");
        }

        HEAP_CHECK(&heap_free[i]);
    }

    return blocks1;
}
