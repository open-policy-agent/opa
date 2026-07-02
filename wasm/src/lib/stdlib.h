#ifndef OPA_STDLIB_H
#define OPA_STDLIB_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

void opa_abort(const char *msg);
__attribute__((import_name("opa_abort"))) void opa_abort_(const char *msg);
void abort(void);
void *malloc(size_t size);
void free(void *ptr);
void *calloc(size_t nmemb, size_t size);
void *realloc(void *ptr, size_t size);

double strtod(const char *nptr, char **endptr);
float strtof(const char *nptr, char **endptr);
long int strtol(const char *nptr, char **endptr, int base);
long long int strtoll(const char *nptr, char **endptr, int base);
unsigned long int strtoul(const char *nptr, char **endptr, int base);
unsigned long long int strtoull(const char *nptr, char **endptr, int base);

typedef struct
{
    int quot;
    int rem;
} div_t;

typedef struct
{
    long int quot;
    long int rem;
} ldiv_t;

typedef struct
{
    long long int quot;
    long long int rem;
} lldiv_t;

// not implemented:

int abs(int j);
long int labs(long int j);
long long int llabs(long long int j);

double atof(const char *nptr);
int atoi(const char *nptr);
long atol(const char *nptr);
long long atoll(const char *nptr);

void *bsearch(const void *key, const void *base,
              size_t nmemb, size_t size,
              int (*compar)(const void *, const void *));

div_t div(int numerator, int denominator);
ldiv_t ldiv(long numerator, long denominator);
lldiv_t lldiv(long long numerator, long long denominator);

void exit(int status);
int atexit(void (*function)(void));
void _Exit(int status);

char *getenv(const char *name);

int mblen(const char *s, size_t n);
size_t mbstowcs(wchar_t *dest, const char *src, size_t n);
int mbtowc(wchar_t *pwc, const char *s, size_t n);

void qsort(void *base, size_t nmemb, size_t size,
           int (*compar)(const void *, const void *));

int rand(void);
int rand_r(unsigned int *seedp);
void srand(unsigned int seed);

long double strtold(const char *nptr, char **endptr);

int system(const char *command);

int wctomb(char *s, wchar_t wc);
size_t wcstombs(char *dest, const wchar_t *src, size_t n);

#ifdef __cplusplus
}
#endif

#endif
