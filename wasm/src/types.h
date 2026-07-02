#ifndef OPA_TYPES_H
#define OPA_TYPES_H

#include "value.h"

opa_value *opa_types_is_number(opa_value *v);
opa_value *opa_types_is_string(opa_value *v);
opa_value *opa_types_is_boolean(opa_value *v);
opa_value *opa_types_is_array(opa_value *v);
opa_value *opa_types_is_set(opa_value *v);
opa_value *opa_types_is_object(opa_value *v);
opa_value *opa_types_is_null(opa_value *v);
opa_value *opa_types_name(opa_value *v);

#endif
