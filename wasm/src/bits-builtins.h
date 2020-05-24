#ifndef OPA_BITS_BUILTINS_H
#define OPA_BITS_BUILTINS_H

#include "value.h"

opa_value *opa_bits_or(opa_value *a, opa_value *b);
opa_value *opa_bits_and(opa_value *a, opa_value *b);
opa_value *opa_bits_negate(opa_value *v);
opa_value *opa_bits_xor(opa_value *a, opa_value *b);
opa_value *opa_bits_shiftleft(opa_value *a, opa_value *b);
opa_value *opa_bits_shiftright(opa_value *a, opa_value *b);

#endif
