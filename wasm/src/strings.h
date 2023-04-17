#ifndef OPA_STRINGS_H
#define OPA_STRINGS_H

#include "value.h"

opa_value *opa_strings_any_prefix_match(opa_value *a, opa_value *b);
opa_value *opa_strings_any_suffix_match(opa_value *a, opa_value *b);
opa_value *opa_strings_concat(opa_value *a, opa_value *b);
opa_value *opa_strings_contains(opa_value *a, opa_value *b);
opa_value *opa_strings_endswith(opa_value *a, opa_value *b);
opa_value *opa_strings_format_int(opa_value *a, opa_value *b);
opa_value *opa_strings_indexof(opa_value *a, opa_value *b);
opa_value *opa_strings_lower(opa_value *a);
opa_value *opa_strings_replace(opa_value *a, opa_value *b, opa_value *c);
opa_value *opa_strings_replace_n(opa_value *a, opa_value *b);
opa_value *opa_strings_reverse(opa_value *a);
opa_value *opa_strings_split(opa_value *a, opa_value *b);
opa_value *opa_strings_startswith(opa_value *a, opa_value *b);
opa_value *opa_strings_substring(opa_value *a, opa_value *b, opa_value *c);
opa_value *opa_strings_trim(opa_value *a, opa_value *b);
opa_value *opa_strings_trim_left(opa_value *a, opa_value *b);
opa_value *opa_strings_trim_prefix(opa_value *a, opa_value *b);
opa_value *opa_strings_trim_right(opa_value *a, opa_value *b);
opa_value *opa_strings_trim_suffix(opa_value *a, opa_value *b);
opa_value *opa_strings_trim_space(opa_value *a);
opa_value *opa_strings_upper(opa_value *a);

#endif
