#ifndef OPA_ENCODING_H
#define OPA_ENCODING_H

#include "value.h"

opa_value *opa_base64_is_valid(opa_value *a);
opa_value *opa_base64_decode(opa_value *a);
opa_value *opa_base64_encode(opa_value *a);
opa_value *opa_base64_url_decode(opa_value *a);
opa_value *opa_base64_url_encode(opa_value *a);
opa_value *opa_json_unmarshal(opa_value *a);
opa_value *opa_json_marshal(opa_value *a);

#endif
