#ifndef OPA_VALUE_H
#define OPA_VALUE_H

#include "std.h"

#define OPA_NULL (1)
#define OPA_BOOLEAN (2)
#define OPA_NUMBER (3)
#define OPA_STRING (4)
#define OPA_ARRAY (5)
#define OPA_OBJECT (6)
#define OPA_SET (7)

#define OPA_EXT_INT (1)
#define OPA_EXT_FLOAT (2)

typedef struct opa_value opa_value;

struct opa_value
{
    unsigned char type;
};

typedef struct
{
    opa_value hdr;
    int v;
} opa_boolean_t;

typedef struct
{
    opa_value hdr;
    unsigned char is_float;
    union {
        long long i;
        double f;
    } v;
} opa_number_t;

typedef struct
{
    opa_value hdr;
    size_t len;
    const char *v;
} opa_string_t;

typedef struct
{
    opa_value *i;
    opa_value *v;
} opa_array_elem_t;

typedef struct
{
    opa_value hdr;
    opa_array_elem_t *elems;
    size_t len;
    size_t cap;
} opa_array_t;

typedef struct opa_object_elem_t opa_object_elem_t;

struct opa_object_elem_t
{
    opa_value *k;
    opa_value *v;
    opa_object_elem_t *next;
};

typedef struct
{
    opa_value hdr;
    opa_object_elem_t *head;
} opa_object_t;

typedef struct opa_set_elem_t opa_set_elem_t;

struct opa_set_elem_t
{
    opa_value *v;
    opa_set_elem_t *next;
};

typedef struct
{
    opa_value hdr;
    opa_set_elem_t *head;
} opa_set_t;

typedef int (*opa_compare_fn)(opa_value *, opa_value *t);

#define opa_cast_boolean(v) container_of(v, opa_boolean_t, hdr)
#define opa_cast_number(v) container_of(v, opa_number_t, hdr)
#define opa_cast_string(v) container_of(v, opa_string_t, hdr)
#define opa_cast_array(v) container_of(v, opa_array_t, hdr)
#define opa_cast_object(v) container_of(v, opa_object_t, hdr)
#define opa_cast_set(v) container_of(v, opa_set_t, hdr)

int opa_value_type(opa_value *node);
int opa_value_compare(opa_value *a, opa_value *b);
opa_value *opa_value_get(opa_value *node, opa_value *key);
opa_value *opa_value_iter(opa_value *node, opa_value *prev);
size_t opa_value_length(opa_value *node);
void opa_value_free(opa_value *node);
opa_value *opa_value_merge(opa_value *a, opa_value *b);

long long opa_value_int(opa_value *node);
double opa_value_float(opa_value *node);
const char *opa_value_string(opa_value *node);
int opa_value_boolean(opa_value *node);

opa_value *opa_null();
opa_value *opa_boolean(int v);
opa_value *opa_number_size(size_t v);
opa_value *opa_number_int(long long v);
opa_value *opa_number_float(double v);
opa_value *opa_string(const char *v, size_t len);
opa_value *opa_string_terminated(const char *v);
opa_value *opa_array();
opa_value *opa_array_with_cap(size_t cap);
opa_value *opa_object();
opa_value *opa_set();

void opa_value_boolean_set(opa_value *v, int b);
void opa_value_number_set_int(opa_value *v, long long i);

void opa_array_free(opa_array_t *arr);
void opa_array_append(opa_array_t *arr, opa_value *v);
void opa_array_sort(opa_array_t *arr, opa_compare_fn cmp_fn);

void opa_object_free(opa_object_t *obj);
opa_array_t *opa_object_keys(opa_object_t *obj);
void opa_object_insert(opa_object_t *obj, opa_value *k, opa_value *v);
opa_object_elem_t *opa_object_get(opa_object_t *obj, opa_value *key);
opa_object_elem_t *opa_object_iter(opa_object_t *obj, opa_object_elem_t *prev);

void opa_set_free(opa_set_t *set);
void opa_set_add(opa_set_t *set, opa_value *v);
opa_set_elem_t *opa_set_get(opa_set_t *set, opa_value *v);
opa_set_elem_t *opa_set_iter(opa_set_t *set, opa_set_elem_t *prev);

#endif
