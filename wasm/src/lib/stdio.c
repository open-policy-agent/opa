#include "stdio.h"
#include "printf.h"

#include "../std.h"

struct _FILE {};

FILE _stderr;
FILE _stdout;

FILE *stderr = &_stderr;
FILE *stdout = &_stdout;

int fprintf(FILE *stream, const char * format, ...)
{
    opa_abort("fprintf: not implemented");
    return 0;
}

size_t fwrite(const void *ptr, size_t size, size_t nmemb, FILE *stream)
{
    opa_abort("fwrite: not implemented");
    return 0;
}

int fputc(int c, FILE *stream)
{
    opa_abort("fputc: not implemented");
    return 0;
}

int fputs(const char *s, FILE *stream)
{
    for (size_t i = 0; s[i] != '\0'; i++)
    {
        fputc(s[i], stream);
    }

    return 1;
}

int puts(const char *s)
{
    for (size_t i = 0; s[i] != '\0'; i++)
    {
        fputc(s[i], stdout);
    }

    return 1;
}
