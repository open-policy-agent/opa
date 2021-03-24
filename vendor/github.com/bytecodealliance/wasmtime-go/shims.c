#include "_cgo_export.h"
#include "shims.h"

static wasm_trap_t* trampoline(
   const wasmtime_caller_t *caller,
   void *env,
   const wasm_val_vec_t *args,
   wasm_val_vec_t *results
) {
    return goTrampolineNew((wasmtime_caller_t*) caller, (size_t) env, (wasm_val_vec_t*) args, results);
}

static wasm_trap_t* wrap_trampoline(
   const wasmtime_caller_t *caller,
   void *env,
   const wasm_val_vec_t *args,
   wasm_val_vec_t *results
) {
    return goTrampolineWrap((wasmtime_caller_t*) caller, (size_t) env, (wasm_val_vec_t*) args, results);
}

wasm_func_t *c_func_new_with_env(wasm_store_t *store, wasm_functype_t *ty, size_t env, int wrap) {
  if (wrap)
    return wasmtime_func_new_with_env(store, ty, wrap_trampoline, (void*) env, goFinalizeWrap);
  return wasmtime_func_new_with_env(store, ty, trampoline, (void*) env, goFinalizeNew);
}

wasmtime_error_t *go_wasmtime_func_call(
    wasm_func_t *func,
    const wasm_val_vec_t *args,
    wasm_val_vec_t *results,
    wasm_trap_t **trap
) {
  wasmtime_error_t *ret = wasmtime_func_call(func, args, results, trap);
  return ret;
}

wasm_extern_t* go_caller_export_get(
  const wasmtime_caller_t* caller,
  char *name_ptr,
  size_t name_len
) {
  wasm_byte_vec_t name;
  name.data = name_ptr;
  name.size = name_len;
  return wasmtime_caller_export_get(caller, &name);
}

wasmtime_error_t* go_linker_define(
    wasmtime_linker_t *linker,
    char *module_ptr,
    size_t module_len,
    char *name_ptr,
    size_t name_len,
    wasm_extern_t *item
) {
  wasm_byte_vec_t module;
  module.data = module_ptr;
  module.size = module_len;
  wasm_byte_vec_t name;
  name.data = name_ptr;
  name.size = name_len;
  return wasmtime_linker_define(linker, &module, &name, item);
}

wasmtime_error_t* go_linker_define_instance(
    wasmtime_linker_t *linker,
    char *name_ptr,
    size_t name_len,
    wasm_instance_t *instance
) {
  wasm_byte_vec_t name;
  name.data = name_ptr;
  name.size = name_len;
  return wasmtime_linker_define_instance(linker, &name, instance);
}

wasmtime_error_t* go_linker_define_module(
    wasmtime_linker_t *linker,
    char *name_ptr,
    size_t name_len,
    wasm_module_t *module
) {
  wasm_byte_vec_t name;
  name.data = name_ptr;
  name.size = name_len;
  return wasmtime_linker_module(linker, &name, module);
}

wasmtime_error_t* go_linker_get_default(
    wasmtime_linker_t *linker,
    char *name_ptr,
    size_t name_len,
    wasm_func_t **func
) {
  wasm_byte_vec_t name;
  name.data = name_ptr;
  name.size = name_len;
  return wasmtime_linker_get_default(linker, &name, func);
}

wasmtime_error_t* go_linker_get_one_by_name(
    wasmtime_linker_t *linker,
    char *module_ptr,
    size_t module_len,
    char *name_ptr,
    size_t name_len,
    wasm_extern_t **item
) {
  wasm_byte_vec_t module, name;
  module.data = module_ptr;
  module.size = module_len;
  name.data = name_ptr;
  name.size = name_len;
  return wasmtime_linker_get_one_by_name(linker, &module,&name, item);
}

void go_externref_new_with_finalizer(
    size_t env,
    wasm_val_t *valp
) {
  wasmtime_externref_new_with_finalizer((void*) env, goFinalizeExternref, valp);
}

void go_init_i32(wasm_val_t *val, int32_t i) { val->of.i32 = i; }
void go_init_i64(wasm_val_t *val, int64_t i) { val->of.i64 = i; }
void go_init_f32(wasm_val_t *val, float i) { val->of.f32 = i; }
void go_init_f64(wasm_val_t *val, double i) { val->of.f64 = i; }

int32_t go_get_i32(wasm_val_t *val) { return val->of.i32; }
int64_t go_get_i64(wasm_val_t *val) { return val->of.i64; }
float go_get_f32(wasm_val_t *val) { return val->of.f32; }
double go_get_f64(wasm_val_t *val) { return val->of.f64; }
