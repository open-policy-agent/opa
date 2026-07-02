#ifndef OPA_GLOB_H
#define OPA_GLOB_H

#include "value.h"

#ifdef __cplusplus
extern "C" {
#endif

opa_value *opa_glob_match(opa_value *pattern, opa_value *delimiters, opa_value *match);

#ifdef __cplusplus
}
#endif

#endif
