#ifndef OPA_ARITHMETIC_H
#define OPA_ARITHMETIC_H

#include <mpdecimal.h>

#include "value.h"

opa_value *opa_arith_abs(opa_value *v);
opa_value *opa_arith_round(opa_value *v);
opa_value *opa_arith_ceil(opa_value *v);
opa_value *opa_arith_floor(opa_value *v);
opa_value *opa_arith_plus(opa_value *a, opa_value *b);
opa_value *opa_arith_minus(opa_value *a, opa_value *b);
opa_value *opa_arith_multiply(opa_value *a, opa_value *b);
opa_value *opa_arith_divide(opa_value *a, opa_value *b);
opa_value *opa_arith_rem(opa_value *a, opa_value *b);


#endif
