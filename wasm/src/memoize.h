#ifndef OPA_MEMOIZE_H
#define OPA_MEMOIZE_H

#include "value.h"

void opa_memoize_init(void);
void opa_memoize_push(void);
void opa_memoize_pop(void);
void opa_memoize_insert(int32_t, opa_value *);
opa_value *opa_memoize_get(int32_t);

#endif