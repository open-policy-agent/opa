#ifndef OPA_MALLOC_H
#define OPA_MALLOC_H

#include "std.h"

void *opa_malloc(size_t size);
void opa_free(void *ptr);

unsigned int opa_heap_ptr_get(void);
unsigned int opa_heap_top_get(void);
void opa_heap_ptr_set(unsigned int);
void opa_heap_top_set(unsigned int);

#endif