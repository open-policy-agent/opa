#include "malloc.h"
#include "context.h"

WASM_EXPORT(opa_eval_ctx_new)
opa_eval_ctx_t *opa_eval_ctx_new()
{
    opa_eval_ctx_t *ctx = (opa_eval_ctx_t *)opa_malloc(sizeof(opa_eval_ctx_t));
    ctx->input = NULL;
    ctx->data = NULL;
    ctx->result = NULL;
    ctx->entrypoint = 0;
    return ctx;
}

WASM_EXPORT(opa_eval_ctx_set_input)
void opa_eval_ctx_set_input(opa_eval_ctx_t *ctx, opa_value *v)
{
    ctx->input = v;
}

WASM_EXPORT(opa_eval_ctx_set_data)
void opa_eval_ctx_set_data(opa_eval_ctx_t *ctx, opa_value *v)
{
    ctx->data = v;
}

WASM_EXPORT(opa_eval_ctx_set_entrypoint)
void opa_eval_ctx_set_entrypoint(opa_eval_ctx_t *ctx, int entrypoint)
{
    ctx->entrypoint = entrypoint;
}

WASM_EXPORT(opa_eval_ctx_get_result)
opa_value *opa_eval_ctx_get_result(opa_eval_ctx_t *ctx)
{
    return ctx->result;
}

OPA_INTERNAL
void __force_import_opa_builtins()
{
    opa_builtin0(-1, NULL);
    opa_builtin1(-1, NULL, NULL);
    opa_builtin2(-1, NULL, NULL, NULL);
    opa_builtin3(-1, NULL, NULL, NULL, NULL);
    opa_builtin4(-1, NULL, NULL, NULL, NULL, NULL);
}
