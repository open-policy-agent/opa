#ifndef OPA_STDINT_H
#define OPA_STDINT_H

typedef char                int8_t;
typedef unsigned char       uint8_t;
typedef short               int16_t;
typedef unsigned short      uint16_t;
typedef long                int32_t;
typedef unsigned long       uint32_t;
typedef unsigned long       size_t;
typedef long long           int64_t;
typedef unsigned long long  uint64_t;
typedef long                ptrdiff_t;
typedef long long           intmax_t;
typedef unsigned long       uintptr_t;

#ifndef __cplusplus
#define true    (1)
#define false   (0)
#define bool    int
#endif

#define INT32_MIN   (-0x7fffffff - 1)
#define INT64_MIN   (-0x7fffffffffffffff - 1)

#define INT32_MAX   0x7fffffff
#define INT64_MAX   0x7fffffffffffffff
#define UINT32_MAX  0xffffffff
#define UINT64_MAX  0xffffffffffffffff
#define SIZE_MAX    UINT32_MAX
#define DBL_MAX     (1.79769313486231570815e+308)

#define NULL        (0)
#define TRUE        (1)
#define FALSE       (0)

#endif
