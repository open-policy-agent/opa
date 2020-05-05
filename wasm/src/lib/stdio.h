#ifndef OPA_STDIO_H
#define OPA_STDIO_H

#include <stdarg.h>
#include <stdint.h>

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

#include "printf.h"

#endif

