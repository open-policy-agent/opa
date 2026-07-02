#ifndef OPA_STDIO_H
#define OPA_STDIO_H

#include <stdarg.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define EOF (-1)

struct _FILE;
typedef struct _FILE FILE;
extern FILE *stderr;
extern FILE *stdout;

typedef struct
{
    int32_t __pos;
    int32_t __state;
} fpos_t;

int fprintf(FILE *stream, const char * format, ...);
int fputc(int c, FILE *stream);
int fputs(const char *s, FILE *stream);
size_t fwrite(const void *ptr, size_t size, size_t nmemb, FILE *stream);
int puts(const char *s);

// not implemented:

void clearerr(FILE* stream);
int fclose(FILE *stream);
int feof(FILE* stream);
int ferror(FILE* stream);
FILE* fopen(const char* filename, const char* mode);
int fflush(FILE *stream);
int fgetc(FILE *stream);
int fgetpos(FILE* stream, fpos_t* pos);
int getc(FILE* stream);
char *fgets(char *s, int size, FILE *stream);
size_t fread(void* ptr, size_t size, size_t nmemb, FILE* stream);
FILE* freopen(const char* filename, const char * mode, FILE * stream);
int fscanf(FILE* stream, const char * format, ...);
int fseek(FILE* stream, long offset, int whence);
int fsetpos(FILE* stream, const fpos_t* pos);
long ftell(FILE* stream);
int getchar(void);
void perror(const char* s);
int printf(const char * format, ...);
int putc(int c, FILE* stream);
int putchar(int c);
int remove(const char* filename);
int rename(const char* old, const char* _new);
void rewind(FILE* stream);
int scanf(const char* format, ...);
void setbuf(FILE* stream, char* buf);
int setvbuf(FILE* stream, char* buf, int mode, size_t size);
int sprintf(char* s, const char* format, ...);
int sscanf(const char* s, const char* format, ...);
FILE* tmpfile(void);
char* tmpnam(char* s);
int ungetc(int c, FILE* stream);
int vfprintf(FILE* stream, const char* format, va_list arg);
int vfscanf(FILE* stream, const char* format, va_list arg);
int vprintf(const char* format, va_list arg);
int vscanf(const char* format, va_list arg);
int vsprintf(char* s, const char* format, va_list arg);
int vsscanf(const char* s, const char* format, va_list arg);

#ifdef __cplusplus
}
#endif

#include "printf.h"

#endif

