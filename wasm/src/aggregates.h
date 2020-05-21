#ifndef OPA_AGGREGATES_H
#define OPA_AGGREGATES_H

#include "value.h"

opa_value *opa_agg_count(opa_value *v);
opa_value *opa_agg_sum(opa_value *v);
opa_value *opa_agg_product(opa_value *v);
opa_value *opa_agg_max(opa_value *v);
opa_value *opa_agg_min(opa_value *v);
opa_value *opa_agg_sort(opa_value *v);
opa_value *opa_agg_all(opa_value *v);
opa_value *opa_agg_any(opa_value *v);

#endif
