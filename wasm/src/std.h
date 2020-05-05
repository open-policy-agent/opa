#ifndef OPA_STD_H
#define OPA_STD_H

#define offsetof(st, member) (size_t)(&((st *)0)->member)

#define container_of(ptr, type, member) ({ \
    const typeof( ((type *)0)->member ) *__mptr = (ptr); \
    (type *)( (char *)__mptr - offsetof(type,member) ); })

void opa_abort(const char *msg);
void opa_println(const char *msg);

#endif
