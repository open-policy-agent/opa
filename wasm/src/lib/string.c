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

int memcmp(const void *s1, const void *s2, size_t n)
{
    const unsigned char *p1 = s1;
    const unsigned char *p2 = s2;

    if (p1 == p2)
    {
        return 0;
    }

    while (n--)
    {
        if (*p1 != *p2)
        {
            return *p1 - *p2;
        }

        p1++;
        p2++;
    }

    return 0;
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

void *memset(void *s, int c, unsigned long n)
{
    unsigned char *p = (unsigned char *)s;

    while (n--)
    {
        *p++ = c;
    }

    return s;
}

char *strchr(const char *s, int c)
{
    while (1)
    {
        if (*s == (char)c)
        {
            return (char *)s;
        }
        else if (*s == '\0')
        {
            break;
        }

        s++;
    }

    return NULL;
}

size_t strlen(const char *s)
{
    size_t i = 0;

    for (i = 0; s[i] != '\0'; i++);

    return i;
}
