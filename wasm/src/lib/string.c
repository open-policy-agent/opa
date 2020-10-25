#include <string.h>

#include "../malloc.h"

void *memchr(const void *src, int c, size_t n)
{
    const unsigned char *s = src;

    while (n--)
    {
        if (*s == (unsigned char)c)
	{
            return (void *)s;
        }

        s++;
    }

    return NULL;
}

void *memcpy(void *dest, const void *src, size_t n)
{
    unsigned char *d = dest;
    const unsigned char *s = src;

    for (size_t i = 0; i < n; i++)
    {
        d[i] = s[i];
    }

    return dest;
}

void *memmove(void *dest, const void *src, size_t n)
{
    unsigned char *d = dest;
    const unsigned char *s = src;
    unsigned char *t = opa_malloc(n);

    for (size_t i = 0; i < n; i++)
    {
        t[i] = s[i];
    }

    for (size_t i = 0; i < n; i++)
    {
        d[i] = t[i];
    }

    opa_free(t);

    return dest;
}

size_t strlen(const char *s)
{
    size_t i = 0;

    for (i = 0; s[i] != '\0'; i++);

    return i;
}
