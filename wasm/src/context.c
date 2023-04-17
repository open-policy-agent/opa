#include "malloc.h"
#include "context.h"
#include "stdlib.h"
#include "value.h"
#include "json.h"

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

WASM_EXPORT(opa_eval)
char *opa_eval(void *reserved, int entrypoint, opa_value *data, char *input, uint32_t input_len, uint32_t heap, bool want_value)
{
    if (reserved != NULL) {
        opa_abort("invalid reserved argument");
    }

    opa_heap_ptr_set(heap);
    opa_eval_ctx_t ctx = {
      .entrypoint = entrypoint,
      .data = data,
      .input = opa_value_parse(input, input_len),
    };

    if (eval(&ctx) != 0) {
        opa_abort("eval failed");
    }
    if (want_value) {
        return opa_value_dump(ctx.result);
    }
    return opa_json_dump(ctx.result);
}

// NOTE(sr): Without this attribute set, LLVM would not let this function
// make it into the Wasm module unchanged. We need it there, so the wasm
// compiler in OPA can replace _this_ eval with _its_ eval, compiled from
// rego.
__attribute__((optnone))
int32_t eval(opa_eval_ctx_t *ctx) {
    return 0;
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
