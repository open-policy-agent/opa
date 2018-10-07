#ifndef OPA_MALLOC_H
#define OPA_MALLOC_H

#include "std.h"

void *opa_malloc(size_t size);
void opa_free(void *ptr);

#endif