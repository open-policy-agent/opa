#include "malloc.h"
#include "context.h"

opa_eval_ctx_t *opa_eval_ctx_new()
{
    opa_eval_ctx_t *ctx = (opa_eval_ctx_t *)opa_malloc(sizeof(opa_eval_ctx_t));
    ctx->input = NULL;
    ctx->data = NULL;
    ctx->result = NULL;
    return ctx;
}

void opa_eval_ctx_set_input(opa_eval_ctx_t *ctx, opa_value *v)
{
    ctx->input = v;
}

void opa_eval_ctx_set_data(opa_eval_ctx_t *ctx, opa_value *v)
{
    ctx->data = v;
}

opa_value *opa_eval_ctx_get_result(opa_eval_ctx_t *ctx)
{
    return ctx->result;
}