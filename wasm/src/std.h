#ifndef OPA_STD_H
#define OPA_STD_H

#define NULL        (0)
#define TRUE        (1)
#define FALSE       (0)
#define DBL_MAX     (1.79769313486231570815e+308)

#ifndef __cplusplus
#define true    (1)
#define false   (0)
#define bool    int
#endif

typedef unsigned long       size_t;
typedef unsigned long long  uint64_t;
typedef long                ptrdiff_t;
typedef long long           intmax_t;
typedef unsigned long       uintptr_t;

typedef __builtin_va_list va_list;

#define va_end(v) __builtin_va_end(v)
#define va_start(v,l) __builtin_va_start(v,l)
#define va_arg(v,l) __builtin_va_arg(v,l)

#define offsetof(st, member) (size_t)(&((st *)0)->member)

#define container_of(ptr, type, member) ({ \
    const typeof( ((type *)0)->member ) *__mptr = (ptr); \
    (type *)( (char *)__mptr - offsetof(type,member) ); })

void opa_abort(const char *msg);
void opa_println(const char *msg);

#endif