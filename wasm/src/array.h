#ifndef OPA_ARRAY_H
#define OPA_ARRAY_H

opa_value *opa_array_concat(opa_value *a, opa_value *b);
opa_value *opa_array_slice(opa_value *a, opa_value *i, opa_value *j);
opa_value *opa_array_reverse(opa_value *a);

#endif
