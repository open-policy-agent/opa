#ifndef OPA_REGEX_H
#define OPA_REGEX_H

#include "value.h"

#ifdef __cplusplus
extern "C" {
#endif

opa_value *opa_regex_is_valid(opa_value *v);
opa_value *opa_regex_match(opa_value *pattern, opa_value *value);
opa_value *opa_regex_find_all_string_submatch(opa_value *pattern, opa_value *string, opa_value *number);

#ifdef __cplusplus
}
#endif

#endif
