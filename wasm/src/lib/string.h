#ifndef OPA_STRING_H
#define OPA_STRING_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

void *memchr(const void *s, int c, size_t n);
int memcmp(const void *s1, const void *s2, size_t n);
void *memcpy(void *dest, const void *src, size_t n);
void *memmove(void *dest, const void *src, size_t n);
void *memset(void *s, int c, size_t n);
char *strcpy(char *dest, const char *src);
char *strchr(const char *s, int c);
size_t strlen(const char *s);

#ifdef __cplusplus
}
#endif

#endif
