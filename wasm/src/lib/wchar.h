#ifndef OPA_WCHAR_H
#define OPA_WCHAR_H

#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <time.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef __WINT_TYPE__ wint_t;

typedef struct
{
    int __internal;
} mbstate_t;

#define WEOF (0xffffffffu)

wchar_t *wmemchr(const wchar_t *s, wchar_t c, size_t n);
int wmemcmp(const wchar_t* s1, const wchar_t* s2, size_t n);
wchar_t *wmemmove(wchar_t *dest, const wchar_t *src, size_t n);
wchar_t *wmemcpy(wchar_t *dest, const wchar_t *src, size_t n);
wchar_t *wmemset(wchar_t *wcs, wchar_t wc, size_t n);
size_t wcslen(const wchar_t* s);

// not implemented:

wint_t btowc(int c);
wint_t fgetwc(FILE* stream);
wchar_t* fgetws(wchar_t* s, int n, FILE* stream);
int fwprintf(FILE* stream, const wchar_t* format, ...);
wint_t fputwc(wchar_t c, FILE* stream);
int fputws(const wchar_t* s, FILE* stream);
int fwide(FILE* stream, int mode);
int fwscanf(FILE* stream, const wchar_t* format, ...);
wint_t getwc(FILE* stream);
wint_t getwchar();
int mbsinit(const mbstate_t *ps);
size_t mbrlen(const char *s, size_t n, mbstate_t *ps);
size_t mbrtowc(wchar_t *pwc, const char *s, size_t n, mbstate_t *ps);
size_t mbsrtowcs(wchar_t *dest, const char **src, size_t len, mbstate_t *ps);
wint_t putwc(wchar_t c, FILE* stream);
wint_t putwchar(wchar_t c);
int swprintf(wchar_t* s, size_t n, const wchar_t* format, ...);
int swscanf(const wchar_t* s, const wchar_t* format, ...);
wint_t ungetwc(wint_t c, FILE* stream);
int vfwprintf(FILE* stream, const wchar_t* format, va_list arg);
int vfwscanf(FILE* stream, const wchar_t* format, va_list arg);
int vswprintf(wchar_t* s, size_t n, const wchar_t* format, va_list arg);
int vswscanf(const wchar_t* s, const wchar_t* format, va_list arg);
int vwprintf(const wchar_t *format, va_list args);
int vwscanf(const wchar_t* format, va_list arg);
size_t wcrtomb(char *s, wchar_t wc, mbstate_t *ps);
wchar_t* wcscat(wchar_t* s1, const wchar_t* s2);
wchar_t *wcschr(const wchar_t *wcs, wchar_t wc);
int wcscoll(const wchar_t* s1, const wchar_t* s2);
int wcscmp(const wchar_t* s1, const wchar_t* s2);
wchar_t* wcscpy(wchar_t* s1, const wchar_t* s2);
size_t wcscspn(const wchar_t* s1, const wchar_t* s2);
size_t wcsftime(wchar_t* s, size_t maxsize, const wchar_t* format, const tm* timeptr);
wchar_t* wcsncat(wchar_t* s1, const wchar_t* s2, size_t n);
int wcsncmp(const wchar_t* s1, const wchar_t* s2, size_t n);
wchar_t* wcsncpy(wchar_t* s1, const wchar_t* s2, size_t n);
wchar_t *wcspbrk(const wchar_t *wcs, const wchar_t *accept);
wchar_t *wcsrchr(const wchar_t *wcs, wchar_t wc);
size_t wcsrtombs(char *dest, const wchar_t **src, size_t len, mbstate_t *ps);
size_t wcsspn(const wchar_t *wcs, const wchar_t *accept);
wchar_t *wcsstr(const wchar_t *haystack, const wchar_t *needle);
wchar_t *wcstok(wchar_t *wcs, const wchar_t *delim, wchar_t **ptr);
size_t wcsxfrm(wchar_t* s1, const wchar_t* s2, size_t n);
int wctob(wint_t c);
int wprintf(const wchar_t* format, ...);
int wscanf(const wchar_t* format, ...);

float wcstof(const wchar_t* nptr, wchar_t** endptr);
double wcstod(const wchar_t* nptr, wchar_t** endptr);
long wcstol(const wchar_t* nptr, wchar_t** endptr, int base);

long double wcstold(const wchar_t* nptr, wchar_t** endptr);
long long wcstoll(const wchar_t* nptr, wchar_t** endptr, int base);

unsigned long wcstoul(const wchar_t* nptr, wchar_t** endptr, int base);
unsigned long long wcstoull(const wchar_t* nptr, wchar_t** endptr, int base);

#ifdef __cplusplus
}
#endif

#endif
