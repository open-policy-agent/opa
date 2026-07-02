#ifndef OPA_MALLOC_H
#define OPA_MALLOC_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

void *opa_malloc(size_t size);
void opa_free(void *ptr);
void *opa_realloc(void *ptr, size_t size);
void opa_free_bulk(void *ptr);
void opa_free_bulk_commit(void);

unsigned int opa_heap_ptr_get(void);
unsigned int opa_heap_top_get(void);
void opa_heap_ptr_set(unsigned int);
void opa_heap_top_set(unsigned int);

void opa_malloc_init(unsigned int);

void opa_malloc_init_test(void);

void *opa_builtin_cache_get(size_t i);
void opa_builtin_cache_set(size_t i, void *p);

size_t opa_heap_free_blocks(void);

void opa_heap_blocks_stash(void);
void opa_heap_blocks_restore(void);
void opa_heap_stash_clear(void);

#ifdef __cplusplus
}
#endif

#endif
