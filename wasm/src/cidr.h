#ifndef OPA_CIDR_H
#define OPA_CIDR_H

#include "value.h"

opa_value *opa_cidr_contains(opa_value *net, opa_value *ip_or_net);
opa_value *opa_cidr_intersects(opa_value *a, opa_value *b);

#endif
