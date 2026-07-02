#ifndef OPA_TIME_H
#define OPA_TIME_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct
{
    int __internal;
} tm;

typedef long int time_t;
typedef long int clock_t;

struct timespec
{
    int __internal;
};

// not implemented:

clock_t clock();
double difftime(time_t time1, time_t time0);
time_t mktime(tm* timeptr);
time_t time(time_t* timer);
char* asctime(const tm* timeptr);
char* ctime(const time_t* timer);
tm* gmtime(const time_t* timer);
tm* localtime(const time_t* timer);
size_t strftime(char* s, size_t maxsize, const char* format, const tm* timeptr);
int timespec_get(struct timespec *ts, int base);

#ifdef __cplusplus
}
#endif

#endif
