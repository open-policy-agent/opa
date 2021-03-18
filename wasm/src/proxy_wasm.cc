// Based on https://github.com/proxy-wasm/proxy-wasm-cpp-sdk/blob/258b4c6974dba5255a9c433450971a56b29228ff/example/http_wasm_example.cc
//
// Copyright 2016-2020 Envoy Project Authors
// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <string>
#include <string_view>
#include <unordered_map>

#include "proxy_wasm_intrinsics.h"
#include "context.h"
#include "json.h"
#include "malloc.h"

class ExampleRootContext : public RootContext {
public:
  explicit ExampleRootContext(uint32_t id, std::string_view root_id) : RootContext(id, root_id) {}

  bool onStart(size_t) override;
  bool onConfigure(size_t) override;
  void onTick() override;
};

class ExampleContext : public Context {
public:
  explicit ExampleContext(uint32_t id, RootContext *root) : Context(id, root) {}

  void onCreate() override;
  FilterHeadersStatus onRequestHeaders(uint32_t headers, bool end_of_stream) override;
  FilterDataStatus onRequestBody(size_t body_buffer_length, bool end_of_stream) override;
  void onDone() override;
  void onLog() override;
  void onDelete() override;

  opa_eval_ctx_t* eval_ctx_;
};

// NOTE(sr): we're calling this to ensure that our implementation here is registered
// with the proxy_wasm_intrinsics.cc globals. The attempt above is suspect.
// This method is called by the module's Start function (_initialize), which is
// wired up in the wasm compiler.
extern "C" OPA_INTERNAL void _register_proxy_wasm(void) {
  RegisterContextFactory register_ExampleContext(CONTEXT_FACTORY(ExampleContext),
                                                 ROOT_FACTORY(ExampleRootContext),
                                                 "opa_root_id");
}

bool ExampleRootContext::onStart(size_t) {
  logInfo("onStart");
  return true;
}

bool ExampleRootContext::onConfigure(size_t) {
  logInfo("onConfigure");
  proxy_set_tick_period_milliseconds(1000); // 1 sec
  return true;
}

void ExampleRootContext::onTick() { logTrace("onTick"); }

void ExampleContext::onCreate() {
  logInfo("onCreate");
  eval_ctx_ = opa_eval_ctx_new();
}

FilterHeadersStatus ExampleContext::onRequestHeaders(uint32_t, bool) {
  logInfo("onRequestHeaders _");
  auto resp = getRequestHeaderPairs();
  auto pairs = resp->pairs();

  opa_object_t *hdrs = opa_cast_object(opa_object());

  // TODO(sr): If a header key is repeated, the second one overwrites the first
  // one. We should figure out if we want to change the input structure, or append
  // those values separated by ","
  for (auto& p : pairs) {
    char *key = (char *) opa_malloc(p.first.size() * sizeof(key));
    char *val = (char *) opa_malloc(p.second.size() * sizeof(val));
    memcpy(key, p.first.data(), p.first.size());
    memcpy(val, p.second.data(), p.second.size());
    opa_object_insert(hdrs, opa_string_allocated(key, p.first.size()), opa_string_allocated(val, p.second.size()));
  }

  opa_object_t *input = opa_cast_object(opa_object());
  opa_object_insert(input, opa_string_terminated("headers"), &hdrs->hdr);
  opa_eval_ctx_set_input(eval_ctx_, &input->hdr);

  logInfo(std::string("input: ") + opa_json_dump(&input->hdr));

  eval(eval_ctx_); // TODO(sr) check return values

  // NOTE(sr): We only look for {"result": true} in the result set.
  // Object returns with custom headers etc would be nice.
  opa_value *res = opa_eval_ctx_get_result(eval_ctx_);
  opa_value *prev = NULL;
  opa_value *curr = NULL;

  while ((curr = opa_value_iter(res, prev)) != NULL)
  {
    opa_value *v = opa_value_get(res, curr);
    logWarn(opa_value_dump(v));
    opa_value *result = opa_value_get(v, opa_string_terminated("result"));
    if (result != NULL) {
      if (opa_value_compare(opa_boolean(true), result) == 0) {
          return FilterHeadersStatus::Continue;
      }
    }
    prev = curr;
  }
  logInfo("no true result, sending local resp");
  sendLocalResponse(403, "OPA policy check denied", "", {});
  return FilterHeadersStatus::StopIteration;
}

// TODO(sr): include in input?
FilterDataStatus ExampleContext::onRequestBody(size_t body_buffer_length,
                                               bool /* end_of_stream */) {
  return FilterDataStatus::Continue;
}

void ExampleContext::onDone() { logInfo("onDone"); }

void ExampleContext::onLog() { logInfo("onLog"); }

void ExampleContext::onDelete() { logInfo("onDelete"); }
