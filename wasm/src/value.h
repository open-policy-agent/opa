#ifndef OPA_VALUE_H
#define OPA_VALUE_H

#include <stdint.h>

#include "std.h"

#ifdef __cplusplus
extern "C" {
#endif

#define OPA_NULL (1)
#define OPA_BOOLEAN (2)
#define OPA_NUMBER (3)
#define OPA_STRING (4)
#define OPA_ARRAY (5)
#define OPA_OBJECT (6)
#define OPA_SET (7)
#define OPA_STRING_INTERNED (8)
#define OPA_BOOLEAN_INTERNED (9) // TODO(sr): make an "interned" bitmask?

#define OPA_NUMBER_REPR_INT (1)
#define OPA_NUMBER_REPR_REF (2)

typedef struct opa_value opa_value;

struct opa_value
{
    unsigned char type;
};

typedef struct
{
    opa_value hdr;
    bool v;
} opa_boolean_t;

typedef struct
{
    const char *s;
    size_t len;
    unsigned char free; // if set 's' is not a reference and should be freed
} opa_number_ref_t;

typedef struct
{
    opa_value hdr;
    unsigned char repr;
    union {
        long long i;
        opa_number_ref_t ref;
    } v;
} opa_number_t;

typedef struct
{
    opa_value hdr;
    unsigned char free; // if set 'v' is not a reference and should be freed
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
    opa_object_elem_t **buckets;
    size_t n;
    size_t len;
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
    opa_set_elem_t **buckets;
    size_t n;
    size_t len;
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
size_t opa_value_hash(opa_value *node);
opa_value *opa_value_get(opa_value *node, opa_value *key);
opa_value *opa_value_iter(opa_value *node, opa_value *prev);
size_t opa_value_length(opa_value *node);
void opa_value_free(opa_value *node);
void opa_value_free_shallow(opa_value *node);
opa_value *opa_value_merge(opa_value *a, opa_value *b);
opa_value *opa_value_shallow_copy(opa_value *node);
opa_value *opa_value_transitive_closure(opa_value *node);
opa_errc opa_value_add_path(opa_value *data, opa_value *path, opa_value *v);
opa_errc opa_value_remove_path(opa_value *data, opa_value *path);

opa_value *opa_null();
opa_value *opa_boolean(bool v);
opa_value *opa_number_size(size_t v);
opa_value *opa_number_int(long long v);
opa_value *opa_number_float(double v);
opa_value *opa_number_ref(const char *s, size_t len);
opa_value *opa_number_ref_allocated(const char *s, size_t len);
void opa_number_init_int(opa_number_t *n, long long v);
opa_value *opa_string(const char *v, size_t len);
opa_value *opa_string_terminated(const char *v);
opa_value *opa_string_allocated(const char *v, size_t len);
opa_value *opa_array();
opa_value *opa_array_with_cap(size_t cap);
opa_value *opa_array_with_elems(opa_array_elem_t *elems, size_t len, size_t cap);
opa_value *opa_object();
opa_value *opa_set();
opa_value *opa_set_with_cap(size_t cap);

void opa_value_number_set_int(opa_value *v, long long i);

int opa_number_try_int(opa_number_t *n, long long *i);
double opa_number_as_float(opa_number_t *n);
void opa_number_free(opa_number_t *n, bool bulk);

void opa_string_free(opa_string_t *s, bool bulk);

void opa_array_free(opa_array_t *arr, bool deep, bool bulk);
void opa_array_append(opa_array_t *arr, opa_value *v);
void opa_array_sort(opa_array_t *arr, opa_compare_fn cmp_fn);

void opa_object_free(opa_object_t *obj, bool deep, bool bulk);
opa_array_t *opa_object_keys(opa_object_t *obj);
void opa_object_insert(opa_object_t *obj, opa_value *k, opa_value *v);
void opa_object_remove(opa_object_t *obj, opa_value *k, bool bulk);
opa_object_elem_t *opa_object_get(opa_object_t *obj, opa_value *key);

void opa_set_free(opa_set_t *set, bool deep, bool bulk);
void opa_set_add(opa_set_t *set, opa_value *v);
opa_set_elem_t *opa_set_get(opa_set_t *set, opa_value *v);

int opa_lookup(opa_value *mapping, opa_value *path);
int opa_mapping_lookup(opa_value *path);
void opa_mapping_init(const char *s, const int l);

#ifdef __cplusplus
}
#endif

#endif
