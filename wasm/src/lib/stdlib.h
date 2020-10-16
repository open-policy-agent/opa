#ifndef OPA_STDLIB_H
#define OPA_STDLIB_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

void abort(void);
void *malloc(size_t size);
void free(void *ptr);
void *calloc(size_t nmemb, size_t size);
void *realloc(void *ptr, size_t size);

long int strtol(const char *nptr, char **endptr, int base);

#ifdef __cplusplus
}
#endif

#endif
