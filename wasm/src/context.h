#ifndef OPA_CONTEXT_H
#define OPA_CONTEXT_H

#include "value.h"

typedef struct
{
    opa_value *input;
    opa_value *data;
    opa_value *result;
    int entrypoint;
} opa_eval_ctx_t;

opa_eval_ctx_t *opa_eval_ctx_new();
void opa_eval_ctx_set_input(opa_eval_ctx_t *ctx, opa_value *v);
void opa_eval_ctx_set_data(opa_eval_ctx_t *ctx, opa_value *v);
void opa_eval_ctx_set_entrypoint(opa_eval_ctx_t *ctx, int entrypoint);
opa_value *opa_eval_ctx_get_result(opa_eval_ctx_t *ctx);

opa_value *opa_builtin0(int, void *);
opa_value *opa_builtin1(int, void *, opa_value *);
opa_value *opa_builtin2(int, void *, opa_value *, opa_value *);
opa_value *opa_builtin3(int, void *, opa_value *, opa_value *, opa_value *);
opa_value *opa_builtin4(int, void *, opa_value *, opa_value *, opa_value *, opa_value *);

int32_t eval(opa_eval_ctx_t *ctx);

#endif