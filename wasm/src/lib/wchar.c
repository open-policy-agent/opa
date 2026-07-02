#include "string.h"

wchar_t *wmemchr(const wchar_t *s, wchar_t c, size_t n)
{
    while (n--)
    {
        if (*s == (wchar_t)c)
        {
            return (wchar_t *)s;
        }

        s++;
    }

    return NULL;
}

int wmemcmp(const wchar_t *s1, const wchar_t *s2, size_t n)
{
    if (s1 == s2)
    {
        return 0;
    }

    while (n--)
    {
        if (*s1 != *s2)
        {
            return *s1 - *s2;
        }

        s1++;
        s2++;
    }

    return 0;
}

wchar_t *wmemcpy(wchar_t *dest, const wchar_t *src, size_t n)
{
    return memcpy(dest, src, n * sizeof(wchar_t));
}

wchar_t *wmemset(wchar_t *wcs, wchar_t wc, size_t n)
{
    wchar_t *p = (wchar_t *)wcs;

    while (n--)
    {
        *p++ = wc;
    }

    return wcs;
}

wchar_t *wmemmove(wchar_t *dest, const wchar_t *src, size_t n)
{
    return memmove(dest, src, n * sizeof(wchar_t));
}

size_t wcslen(const wchar_t *s)
{
    size_t i = 0;

    for (i = 0; s[i] != L'\0'; i++);

    return i;
}
