#ifndef OPA_STD_H
#define OPA_STD_H

#include <stddef.h>

#define container_of(ptr, type, member) ({ \
    const typeof( ((type *)0)->member ) *__mptr = (ptr); \
    (type *)( (char *)__mptr - offsetof(type,member) ); })

void opa_abort(const char *msg);
void opa_println(const char *msg);

#ifdef DEBUG
#include "printf.h"
#define TRACE(...)                               \
    do {                                         \
        char __trace_buf[256];                   \
        snprintf(__trace_buf, 256, __VA_ARGS__); \
        opa_println(__trace_buf);                \
    } while (0)
#else
#define TRACE(...)
#endif

#define TRUE        (1)
#define FALSE       (0)

#ifndef __cplusplus
#define true    (1)
#define false   (0)
#define bool    int
#endif

#endif
