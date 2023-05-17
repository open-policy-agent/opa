#include <string.h>
#include "std.h"
#include "stdlib.h"

#define WASM_PAGE_SIZE (65536)

#define ARRAY_SIZE(ARRAY) (sizeof(ARRAY) / sizeof((ARRAY)[0]))

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

static struct heap_blocks heap_stash[5] = {
    {true, 4},
    {true, 8},
    {true, 16},
    {true, 64},
    {false, 128},
};


/*
 * Currently, there is one variable sized blocklist.  If there were more,
 * we'd need to track each one here in heap_bulk_blocks.
 */
#define VARIABLE_SIZED_BLOCK_IDX (ARRAY_SIZE(heap_free)-1)
static struct heap_blocks heap_bulk_blocks = { false, 128 };
static bool variable_block_update_required = false;


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
    heap_bulk_blocks.start = (struct heap_block) { 0, NULL, &heap_bulk_blocks.end };
    heap_bulk_blocks.end = (struct heap_block) { 0, &heap_bulk_blocks.start, NULL };
    variable_block_update_required = false;

    for (int i = 0; i < ARRAY_SIZE(builtin_cache); i++)
    {
        builtin_cache[i] = NULL;
    }
}

static void init_stash()
{
    for (int i = 0; i < ARRAY_SIZE(heap_stash); i++) {
        heap_stash[i].start =
            (struct heap_block) { 0, NULL, &heap_stash[i].end };
        heap_stash[i].end =
            (struct heap_block) { 0, &heap_stash[i].start, NULL };
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


// NOTE(sr): In internal/compiler/wasm, we append segments to the data section.
// Since our memory layout is
//   |  <-- stack | -- data -- | heap -->  |
// we need to adjust the border between data and heap, i.e., where the heap
// starts. When initializing a module, the Start function emitted by the
// compiler will call this function with the new heap base.
OPA_INTERNAL
void opa_malloc_init(unsigned int heap_base)
{
    heap_ptr = heap_base;
    heap_top = __builtin_wasm_memory_grow(0, 0) * WASM_PAGE_SIZE;
    init_free();
    init_stash();
}

void opa_malloc_init_test(void)
{
    opa_malloc_init(__heap_base);
}

static struct heap_block * __opa_malloc_reuse_fixed(struct heap_blocks *blocks);
static struct heap_block * __opa_malloc_reuse_varying(struct heap_blocks *blocks, size_t size);
static void move_blocks(struct heap_blocks *dst, struct heap_blocks *src);
void opa_free_bulk_commit(void);

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

OPA_INTERNAL
void move_freelists(struct heap_blocks *dst_block_list,
                    struct heap_blocks *src_block_list,
                    const char *caller_fail_msg)
{
    /*
     * First verify that dst freelists are empty and
     * all blocks in the src are below the current heap pointer.
     */
    for (int i = 0; i < ARRAY_SIZE(heap_free); i++)
    {
        struct heap_blocks *dst = &dst_block_list[i];
        struct heap_blocks *src = &src_block_list[i];

        if (dst->start.next != &dst->end || dst->end.prev != &dst->start)
            opa_abort(caller_fail_msg);

        if (src->end.prev != &src->start) {
            struct heap_block *b = src->end.prev;
            if ((unsigned int)b + b->size + sizeof(struct heap_block) > heap_ptr)
                opa_abort(caller_fail_msg);
        }
    }

    /* Now move the blocks en masse from one freelist to the other. */
    for (int i = 0; i < ARRAY_SIZE(heap_free); i++)
        move_blocks(&dst_block_list[i], &src_block_list[i]);
}

WASM_EXPORT(opa_heap_blocks_stash)
void opa_heap_blocks_stash(void)
{
    move_freelists(heap_stash, heap_free,
                   "opa_heap_blocks_stash() consistency check failed");
    /* clean up dangling references */
    init_free();
}

WASM_EXPORT(opa_heap_stash_clear)
void opa_heap_stash_clear(void)
{
    init_stash();
}

WASM_EXPORT(opa_heap_blocks_restore)
void opa_heap_blocks_restore(void)
{
    move_freelists(heap_free, heap_stash,
                   "opa_heap_blocks_restore() consistency check failed");
    /* clean up dangling references */
    init_stash();
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
        if (__builtin_wasm_memory_grow(0, pages) == -1 )
        {
            opa_abort("opa_malloc: failed");
        };
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

    if (variable_block_update_required)
        opa_free_bulk_commit();

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

    if (!fixed_size)
    {
        for (struct heap_block *b = prev->next; b < block && b != end; prev = b, b = b->next);

        struct heap_block *prev_end = (void *)(&prev->data[0]) + prev->size;
        struct heap_block *block_end = (void *)(&block->data[0]) + block->size;

        if (prev_end == block)
        {
            prev->size += sizeof(struct heap_block) + block->size;
            prev_end = (void *)(&prev->data[0]) + prev->size;
            if (prev_end == prev->next) {
                struct heap_block *next = prev->next;
                prev->size += sizeof(struct heap_block) + next->size;
                prev->next = next->next;
                prev->next->prev = prev;
            }
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
            return;
        }
    }

    // List the block as free.

    block->prev = prev;
    block->next = prev->next;
    prev->next = block;
    block->next->prev = block;
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

// Count the number of free blocks. This is for testing only.
size_t opa_heap_free_blocks(void)
{
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

static bool blocks_empty(struct heap_blocks *blocks)
{
    return blocks->start.next == &blocks->end;
}

static void init_blocks(struct heap_blocks *blocks)
{
    blocks->start = (struct heap_block) { 0, NULL, &blocks->end };
    blocks->end = (struct heap_block) { 0, &blocks->start, NULL };
}

static void remove_block(struct heap_block *block)
{
    block->prev->next = block->next;
    block->next->prev = block->prev;
    block->prev = NULL;
    block->next = NULL;
}

static void append_block(struct heap_blocks *blocks, struct heap_block *block)
{
    block->prev = blocks->end.prev;
    block->next = &blocks->end;
    block->prev->next = block;
    block->next->prev = block;
}

static void prepend_block(struct heap_blocks *blocks, struct heap_block *block)
{
    block->prev = &blocks->start;
    block->next = blocks->start.next;
    block->prev->next = block;
    block->next->prev = block;
}

static void move_blocks(struct heap_blocks *dst, struct heap_blocks *src)
{
    dst->start.prev = NULL; /* unnecessary, but safe */
    dst->start.next = src->start.next;
    dst->start.next->prev = &dst->start;

    dst->end.prev = src->end.prev;
    dst->end.next = NULL; /* unnecessary, but safe */
    dst->end.prev->next = &dst->end;

    /* Fix dangling references in src for consistency */
    src->start.next = &src->end;
    src->end.prev = &src->start;
}

static void merge_or_append_block(struct heap_blocks *dst, struct heap_block *block)
{
    struct heap_block *last = dst->end.prev;
    struct heap_block *last_end = (void *)(&last->data[0]) + last->size;
    if (last != &dst->start && last_end == block)
        last->size += sizeof(struct heap_block) + block->size;
    else
        append_block(dst, block);
}

static void merge_or_append_blocks(struct heap_blocks *dst, struct heap_blocks *src)
{
    while (!blocks_empty(src))
    {
        struct heap_block *block = src->start.next;
        remove_block(block);
        merge_or_append_block(dst, block);
    }
}

/*
 * Assumes list1 and list2 are in order.  Merge them into dst in order.
 * dst, list1 and list2 must all be different block lists. Assumes dst has
 * been initialized.
 *
 * As a special case, if two blocks being merged are adjacent, combine them
 * into a single block.
 */
static void merge_blocks(struct heap_blocks *dst, struct heap_blocks *list1,
                         struct heap_blocks *list2)
{
    while (!blocks_empty(list1) && !blocks_empty(list2)) {
        struct heap_block *b1 = list1->start.next;
        struct heap_block *b2 = list2->start.next;
        struct heap_block *min = (unsigned int)b1 < (unsigned int)b2 ? b1 : b2;
        remove_block(min);
        merge_or_append_block(dst, min);
    }

    /* at most one list still has blocks */
    if (!blocks_empty(list1))
        merge_or_append_blocks(dst, list1);
    else
        merge_or_append_blocks(dst, list2);
}

/*
 * Split a list of blocks into two by alternately appending the blocks
 * to two separate block lists. Assumes dst[0] and dst[1] have been initialized.
 */
static void split_blocks(struct heap_blocks dst[2], struct heap_blocks *src)
{
    unsigned int i = 0;
    while (!blocks_empty(src)) {
        struct heap_block *block = src->start.next;
        remove_block(block);
        append_block(&dst[i], block);
        i ^= 1;
    }
}

/* Merge sort the blocks on a list in ascending address order */
void merge_sort_blocks(struct heap_blocks *blocks)
{
    struct heap_blocks hold[2];
    struct heap_block *first;
    struct heap_block *second;

    /* list length == 0: done */
    if (blocks_empty(blocks))
        return;

    first = blocks->start.next;
    second = first->next;

    /* list length == 1: done */
    if (second == &blocks->end)
        return;

    /* list length == 2: optimization -- fast block swap+merge */
    if (second->next == &blocks->end)
    {
        if ((unsigned int)first > (unsigned int)second)
        {
            remove_block(first);
            /* blocks now just has 'second' to which we append or merge 'first' */
            merge_or_append_block(blocks, first);
        }
        /* first and second are in order.  See if we can merge them. */
        else if (((void *)(&first->data[0]) + first->size == second))
        {
            remove_block(second);
            first->size += sizeof(struct heap_block) + second->size;
        }

        /* one way or the other, we're done */
        return;
    }

    /* list length > 2: recursive case */
    for (int i = 0; i < 2; i++)
        init_blocks(&hold[i]);
    split_blocks(hold, blocks);
    merge_sort_blocks(&hold[0]);
    merge_sort_blocks(&hold[1]);
    merge_blocks(blocks, &hold[0], &hold[1]);
}

static void block_order_check(struct heap_blocks *blocks)
{
    struct heap_block *b;
    struct heap_block *prev;
    struct heap_block *prev_end;

    for (prev = NULL, b = blocks->start.next ; b != &blocks->end; prev = b, b = b->next)
    {
        if (prev == NULL)
            continue;
	prev_end = (void *)(&prev->data[0]) + prev->size;
        if ((unsigned int)prev >= (unsigned int)b)
            opa_abort("block_order_check() out of order blocks detected");
        if (prev_end > b)
            opa_abort("block_order_check() overlapping blocks detected");
        if (!blocks->fixed_size && prev_end == b)
            opa_abort("block_order_check() unmerged block detected");
    }
}

#ifndef DEBUG
#define BLOCK_ORDER_CHECK(blocks)
#else /* DEBUG */
#define BLOCK_ORDER_CHECK(blocks) block_order_check(blocks)
#endif /* DEBUG */

/*
 * Save heap blocks to temporary block lists in arbitrary order.
 * Later, in opa_free_bulk_commit() release them correctly back to the heap.
 */
void opa_free_bulk(void *ptr)
{
    struct heap_block *block = ptr - sizeof(struct heap_block);
    struct heap_blocks *blocks = __opa_blocks(block->size);

#ifdef DEBUG
    if (ptr == NULL)
    {
        opa_abort("opa_free_bulk: null pointer");
    }

    if (block->prev != NULL || block->next != NULL)
    {
        opa_abort("opa_free_bulk: double free");
    }
#endif

    if (blocks->fixed_size) {
        prepend_block(blocks, block);
        HEAP_CHECK(blocks);
    } else {
        prepend_block(&heap_bulk_blocks, block);
        HEAP_CHECK(&heap_bulk_blocks);
        variable_block_update_required = true;
    }
}

/*
 * Return the variable-sized blocks released by opa_free_bulk() to the heap.
 * opa_free_bulk() placed the blocks on a list but disregarded
 * address order unlike what is done in the heap.  This makes freeing K objects
 * take O(K) time. Now, to return them to the heap, we need to put them
 * in address order along with the other blocks on the heap.  This takes
 * O(K log K) time where K == N + <number of free blocks>.
 *
 * This is in contrast to iterative calls to opa_free(). Each call to opa_free()
 * takes a worst-case of O(N) time due to the time it takes to linearly insert
 * the block into the list.  Calling opa_free() iteratively over N objects,
 * threfore, takes time that grows in O(N^2).  (Even with an empty freelist,
 * the average length of the search is O(N/2)).
 *
 * This function should generally be private.  However it is exposed in the
 * malloc.h header in case there is a specific desire to ensure predictability
 * of allocation time after some bulk free operations.  It is also useful
 * for tests.
 */
void opa_free_bulk_commit(void)
{
    struct heap_blocks *var_blocks = &heap_free[VARIABLE_SIZED_BLOCK_IDX];
    struct heap_blocks hold;

    merge_sort_blocks(&heap_bulk_blocks);
    BLOCK_ORDER_CHECK(&heap_bulk_blocks);

    /*
     * Need to move variable blocks to new list because merge_blocks()
     * expects three distinct lists.
     */
    init_blocks(&hold);
    move_blocks(&hold, var_blocks);
    merge_blocks(var_blocks, &heap_bulk_blocks, &hold);

#ifdef DEBUG
    if (!blocks_empty(&heap_bulk_blocks))
        opa_abort("Unmerged bulk blocks in heap_bulk_blocks");
    if (!blocks_empty(&hold))
        opa_abort("Unmerged heap blocks in heap_bulk_blocks");
#endif /* DEBUG */
    HEAP_CHECK(var_blocks);
    BLOCK_ORDER_CHECK(var_blocks);

    variable_block_update_required = false;
}
