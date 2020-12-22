#ifndef OPA_OBJECT_H
#define OPA_OBJECT_H

#include "value.h"

opa_value *builtin_object_filter(opa_value *obj, opa_value *keys);
opa_value *builtin_object_get(opa_value *obj, opa_value *key, opa_value *value);
opa_value *builtin_object_remove(opa_value *obj, opa_value *keys);
opa_value *builtin_object_union(opa_value *a, opa_value *b);

#endif
