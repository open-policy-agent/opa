#include <wasm.h>
#include <wasmtime.h>

wasm_func_t *c_func_new_with_env(wasm_store_t *store, wasm_functype_t *ty, size_t env, int wrap);
wasm_extern_t* go_caller_export_get(const wasmtime_caller_t* caller, char *name_ptr, size_t name_len);
wasmtime_error_t* go_linker_define(
    wasmtime_linker_t *linker,
    char *module_ptr,
    size_t module_len,
    char *name_ptr,
    size_t name_len,
    wasm_extern_t *item
);
wasmtime_error_t* go_linker_define_instance(
    wasmtime_linker_t *linker,
    char *name_ptr,
    size_t name_len,
    wasm_instance_t *item
);
wasmtime_error_t* go_linker_define_module(
    wasmtime_linker_t *linker,
    char *name_ptr,
    size_t name_len,
    wasm_module_t *item
);
wasmtime_error_t* go_linker_get_default(
    wasmtime_linker_t *linker,
    char *name_ptr,
    size_t name_len,
    wasm_func_t **func
);
wasmtime_error_t* go_linker_get_one_by_name(
    wasmtime_linker_t *linker,
    char *module_ptr,
    size_t module_len,
    char *name_ptr,
    size_t name_len,
    wasm_extern_t **item
);
void go_externref_new_with_finalizer(
    size_t env,
    wasm_val_t *valp
);
wasmtime_error_t *go_wasmtime_func_call(
    wasm_func_t *func,
    const wasm_val_vec_t *args,
    wasm_val_vec_t *results,
    wasm_trap_t **trap
);
void go_init_i32(wasm_val_t *val, int32_t i);
void go_init_i64(wasm_val_t *val, int64_t i);
void go_init_f32(wasm_val_t *val, float i);
void go_init_f64(wasm_val_t *val, double i);

int32_t go_get_i32(wasm_val_t *val);
int64_t go_get_i64(wasm_val_t *val);
float go_get_f32(wasm_val_t *val);
double go_get_f64(wasm_val_t *val);
