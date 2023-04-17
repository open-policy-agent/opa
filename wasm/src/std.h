#ifndef OPA_STD_H
#define OPA_STD_H

#include <stdbool.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

#define container_of(ptr, type, member)                             \
    ((type *)(void *)( ((char *)(ptr) - offsetof(type, member) )))

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

// Functions to be exported from the WASM module
#define WASM_EXPORT(NAME) __attribute__((export_name(#NAME)))
// functions that implement builtins
#define OPA_BUILTIN __attribute__((used))
// functions that may be called from the generated WASM code
#define OPA_INTERNAL __attribute__((used))

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

