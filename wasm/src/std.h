#ifndef OPA_STD_H
#define OPA_STD_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

#define container_of(ptr, type, member)                             \
    ((type *)(void *)( ((char *)(ptr) - offsetof(type, member) )))

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

// OPA WASM API Error Codes
#define OPA_ERR_OK 0
#define OPA_ERR_INTERNAL 1
#define OPA_ERR_INVALID_TYPE 2
#define OPA_ERR_INVALID_PATH 3

typedef int opa_errc;

#ifdef __cplusplus
}
#endif

#endif
