#ifndef OPA_MALLOC_H
#define OPA_MALLOC_H

#include <stddef.h>

void *opa_malloc(size_t size);
void opa_free(void *ptr);
void *opa_realloc(void *ptr, size_t size);

unsigned int opa_heap_ptr_get(void);
unsigned int opa_heap_top_get(void);
void opa_heap_ptr_set(unsigned int);
void opa_heap_top_set(unsigned int);

size_t opa_heap_free_blocks(void);

#endif
