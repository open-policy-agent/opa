#ifndef OPA_SET_H
#define OPA_SET_H

#include "value.h"

opa_value *opa_set_diff(opa_value *a, opa_value *b);
opa_value *opa_set_intersection(opa_value *a, opa_value *b);
opa_value *opa_set_union(opa_value *a, opa_value *b);

opa_value *opa_sets_intersection(opa_value *v);
opa_value *opa_sets_union(opa_value *v);

#endif
