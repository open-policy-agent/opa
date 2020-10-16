#ifndef OPA_STDIO_H
#define OPA_STDIO_H

#include <stdarg.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

struct _FILE;
typedef struct _FILE FILE;
extern FILE *stderr;
extern FILE *stdout;

int fprintf(FILE *stream, const char * format, ...);
int fputc(int c, FILE *stream);
int fputs(const char *s, FILE *stream);
size_t fwrite(const void *ptr, size_t size, size_t nmemb, FILE *stream);
int printf(const char * format, ...);
int puts(const char *s);

#ifdef __cplusplus
}
#endif

#include "printf.h"

#endif

