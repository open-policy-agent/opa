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
    if (stream != stderr && stream != stdout)
    {
        opa_abort("fprintf: only stdout and stderr allowed");
    }

    char buf[256];
    va_list arg;

    va_start(arg, format);
    vsnprintf_(buf, 256, format, arg); // TODO: Remove character limit and extra \n.
    opa_println(buf);
    return 0;
}

size_t fwrite(const void *ptr, size_t size, size_t nmemb, FILE *stream)
{
    if (stream != stderr && stream != stdout)
    {
        opa_abort("fwrite: only stdout and stderr allowed");
    }

    const char *s = ptr;

    for (size_t i = 0; i < size; )
    {
        char buf[80];
        i += snprintf(buf, sizeof(buf), "%s", &s[i]);
        opa_println(buf); // TODO: Remove the extra \n.
    }

    return size;
}

int fputc(int c, FILE *stream)
{
    if (stream != stderr && stream != stdout)
    {
        opa_abort("fputc: only stdout and stderr allowed");
    }

    char buf[2];
    snprintf(buf, sizeof(buf), "%c", c);
    opa_println(buf); // TODO: Remove the extra \n.
    return c;
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
