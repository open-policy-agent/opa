const __vite__mapDeps=(i,m=__vite__mapDeps,d=(m.f||(m.f=["assets/inputs-vIc-jKci.js","assets/inputs-CdzzfXLo.css"])))=>i.map(i=>d[i]);
(function polyfill() {
  const relList = document.createElement("link").relList;
  if (relList && relList.supports && relList.supports("modulepreload")) return;
  for (const link of document.querySelectorAll('link[rel="modulepreload"]')) processPreload(link);
  new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      if (mutation.type !== "childList") continue;
      for (const node of mutation.addedNodes) if (node.tagName === "LINK" && node.rel === "modulepreload") processPreload(node);
    }
  }).observe(document, {
    childList: true,
    subtree: true
  });
  function getFetchOpts(link) {
    const fetchOpts = {};
    if (link.integrity) fetchOpts.integrity = link.integrity;
    if (link.referrerPolicy) fetchOpts.referrerPolicy = link.referrerPolicy;
    if (link.crossOrigin === "use-credentials") fetchOpts.credentials = "include";
    else if (link.crossOrigin === "anonymous") fetchOpts.credentials = "omit";
    else fetchOpts.credentials = "same-origin";
    return fetchOpts;
  }
  function processPreload(link) {
    if (link.ep) return;
    link.ep = true;
    const fetchOpts = getFetchOpts(link);
    fetch(link.href, fetchOpts);
  }
})();
class RuntimeError extends Error {
  constructor(message, input2) {
    super(message);
    this.input = input2;
  }
}
RuntimeError.prototype.name = "RuntimeError";
function generatorish(value) {
  return value && typeof value.next === "function" && typeof value.return === "function";
}
function constant(x) {
  return () => x;
}
function identity(x) {
  return x;
}
function rethrow(error) {
  return () => {
    throw error;
  };
}
const prototype = Array.prototype;
const map = prototype.map;
function noop() {
}
const TYPE_NORMAL = 1;
const TYPE_IMPLICIT = 2;
const TYPE_DUPLICATE = 3;
const no_observer = Symbol("no-observer");
const no_value = Promise.resolve();
function Variable(type, module, observer, options) {
  if (!observer) observer = no_observer;
  Object.defineProperties(this, {
    _observer: { value: observer, writable: true },
    _definition: { value: variable_undefined, writable: true },
    _duplicate: { value: void 0, writable: true },
    _duplicates: { value: void 0, writable: true },
    _indegree: { value: NaN, writable: true },
    // The number of computing inputs.
    _inputs: { value: [], writable: true },
    _invalidate: { value: noop, writable: true },
    _module: { value: module },
    _name: { value: null, writable: true },
    _outputs: { value: /* @__PURE__ */ new Set(), writable: true },
    _promise: { value: no_value, writable: true },
    _reachable: { value: observer !== no_observer, writable: true },
    // Is this variable transitively visible?
    _rejector: { value: variable_rejector(this) },
    _shadow: { value: initShadow(module, options) },
    _type: { value: type },
    _value: { value: void 0, writable: true },
    _version: { value: 0, writable: true }
  });
}
Object.defineProperties(Variable.prototype, {
  _pending: { value: variable_pending, writable: true, configurable: true },
  _fulfilled: { value: variable_fulfilled, writable: true, configurable: true },
  _rejected: { value: variable_rejected, writable: true, configurable: true },
  _resolve: { value: variable_resolve, writable: true, configurable: true },
  define: { value: variable_define, writable: true, configurable: true },
  delete: { value: variable_delete, writable: true, configurable: true },
  import: { value: variable_import, writable: true, configurable: true }
});
function initShadow(module, options) {
  if (!options?.shadow) return null;
  return new Map(
    Object.entries(options.shadow).map(([name, definition]) => [name, new Variable(TYPE_IMPLICIT, module).define([], definition)])
  );
}
function variable_attach(variable) {
  variable._module._runtime._dirty.add(variable);
  variable._outputs.add(this);
}
function variable_detach(variable) {
  variable._module._runtime._dirty.add(variable);
  variable._outputs.delete(this);
}
function variable_undefined() {
  throw variable_undefined;
}
function variable_stale() {
  throw variable_stale;
}
function variable_rejector(variable) {
  return (error) => {
    if (error === variable_stale) throw error;
    if (error === variable_undefined) throw new RuntimeError(`${variable._name} is not defined`, variable._name);
    if (error instanceof Error && error.message) throw new RuntimeError(error.message, variable._name);
    throw new RuntimeError(`${variable._name} could not be resolved`, variable._name);
  };
}
function variable_duplicate(name) {
  return () => {
    throw new RuntimeError(`${name} is defined more than once`);
  };
}
function variable_define(name, inputs, definition) {
  switch (arguments.length) {
    case 1: {
      definition = name, name = inputs = null;
      break;
    }
    case 2: {
      definition = inputs;
      if (typeof name === "string") inputs = null;
      else inputs = name, name = null;
      break;
    }
  }
  return variable_defineImpl.call(
    this,
    name == null ? null : String(name),
    inputs == null ? [] : map.call(inputs, this._resolve, this),
    typeof definition === "function" ? definition : constant(definition)
  );
}
function variable_resolve(name) {
  return this._shadow?.get(name) ?? this._module._resolve(name);
}
function variable_defineImpl(name, inputs, definition) {
  const scope = this._module._scope, runtime = this._module._runtime;
  this._inputs.forEach(variable_detach, this);
  inputs.forEach(variable_attach, this);
  this._inputs = inputs;
  this._definition = definition;
  this._value = void 0;
  if (definition === noop) runtime._variables.delete(this);
  else runtime._variables.add(this);
  if (name !== this._name || scope.get(name) !== this) {
    let error, found;
    if (this._name) {
      if (this._outputs.size) {
        scope.delete(this._name);
        found = this._module._resolve(this._name);
        found._outputs = this._outputs, this._outputs = /* @__PURE__ */ new Set();
        found._outputs.forEach(function(output) {
          output._inputs[output._inputs.indexOf(this)] = found;
        }, this);
        found._outputs.forEach(runtime._updates.add, runtime._updates);
        runtime._dirty.add(found).add(this);
        scope.set(this._name, found);
      } else if ((found = scope.get(this._name)) === this) {
        scope.delete(this._name);
      } else if (found._type === TYPE_DUPLICATE) {
        found._duplicates.delete(this);
        this._duplicate = void 0;
        if (found._duplicates.size === 1) {
          found = found._duplicates.keys().next().value;
          error = scope.get(this._name);
          found._outputs = error._outputs, error._outputs = /* @__PURE__ */ new Set();
          found._outputs.forEach(function(output) {
            output._inputs[output._inputs.indexOf(error)] = found;
          });
          found._definition = found._duplicate, found._duplicate = void 0;
          runtime._dirty.add(error).add(found);
          runtime._updates.add(found);
          scope.set(this._name, found);
        }
      } else {
        throw new Error();
      }
    }
    if (this._outputs.size) throw new Error();
    if (name) {
      if (found = scope.get(name)) {
        if (found._type === TYPE_DUPLICATE) {
          this._definition = variable_duplicate(name), this._duplicate = definition;
          found._duplicates.add(this);
        } else if (found._type === TYPE_IMPLICIT) {
          this._outputs = found._outputs, found._outputs = /* @__PURE__ */ new Set();
          this._outputs.forEach(function(output) {
            output._inputs[output._inputs.indexOf(found)] = this;
          }, this);
          runtime._dirty.add(found).add(this);
          scope.set(name, this);
        } else {
          found._duplicate = found._definition, this._duplicate = definition;
          error = new Variable(TYPE_DUPLICATE, this._module);
          error._name = name;
          error._definition = this._definition = found._definition = variable_duplicate(name);
          error._outputs = found._outputs, found._outputs = /* @__PURE__ */ new Set();
          error._outputs.forEach(function(output) {
            output._inputs[output._inputs.indexOf(found)] = error;
          });
          error._duplicates = /* @__PURE__ */ new Set([this, found]);
          runtime._dirty.add(found).add(error);
          runtime._updates.add(found).add(error);
          scope.set(name, error);
        }
      } else {
        scope.set(name, this);
      }
    }
    this._name = name;
  }
  if (this._version > 0) ++this._version;
  runtime._updates.add(this);
  runtime._compute();
  return this;
}
function variable_import(remote, name, module) {
  if (arguments.length < 3) module = name, name = remote;
  return variable_defineImpl.call(this, String(name), [module._resolve(String(remote))], identity);
}
function variable_delete() {
  return variable_defineImpl.call(this, null, [], noop);
}
function variable_pending() {
  if (this._observer.pending) this._observer.pending();
}
function variable_fulfilled(value) {
  if (this._observer.fulfilled) this._observer.fulfilled(value, this._name);
}
function variable_rejected(error) {
  if (this._observer.rejected) this._observer.rejected(error, this._name);
}
const variable_variable = Symbol("variable");
const variable_invalidation = Symbol("invalidation");
const variable_visibility = Symbol("visibility");
function Module(runtime, builtins = []) {
  Object.defineProperties(this, {
    _runtime: { value: runtime },
    _scope: { value: /* @__PURE__ */ new Map() },
    _builtins: { value: new Map([
      ["@variable", variable_variable],
      ["invalidation", variable_invalidation],
      ["visibility", variable_visibility],
      ...builtins
    ]) },
    _source: { value: null, writable: true }
  });
}
Object.defineProperties(Module.prototype, {
  _resolve: { value: module_resolve, writable: true, configurable: true },
  redefine: { value: module_redefine, writable: true, configurable: true },
  define: { value: module_define, writable: true, configurable: true },
  derive: { value: module_derive, writable: true, configurable: true },
  import: { value: module_import, writable: true, configurable: true },
  value: { value: module_value, writable: true, configurable: true },
  variable: { value: module_variable, writable: true, configurable: true },
  builtin: { value: module_builtin, writable: true, configurable: true }
});
function module_redefine(name) {
  const v = this._scope.get(name);
  if (!v) throw new RuntimeError(`${name} is not defined`);
  if (v._type === TYPE_DUPLICATE) throw new RuntimeError(`${name} is defined more than once`);
  return v.define.apply(v, arguments);
}
function module_define() {
  const v = new Variable(TYPE_NORMAL, this);
  return v.define.apply(v, arguments);
}
function module_import() {
  const v = new Variable(TYPE_NORMAL, this);
  return v.import.apply(v, arguments);
}
function module_variable(observer, options) {
  return new Variable(TYPE_NORMAL, this, observer, options);
}
async function module_value(name) {
  let v = this._scope.get(name);
  if (!v) throw new RuntimeError(`${name} is not defined`);
  if (v._observer === no_observer) {
    v = this.variable(true).define([name], identity);
    try {
      return await module_revalue(this._runtime, v);
    } finally {
      v.delete();
    }
  } else {
    return module_revalue(this._runtime, v);
  }
}
async function module_revalue(runtime, variable) {
  await runtime._compute();
  try {
    return await variable._promise;
  } catch (error) {
    if (error === variable_stale) return module_revalue(runtime, variable);
    throw error;
  }
}
function module_derive(injects, injectModule) {
  const map2 = /* @__PURE__ */ new Map();
  const modules = /* @__PURE__ */ new Set();
  const copies = [];
  function alias(source) {
    let target = map2.get(source);
    if (target) return target;
    target = new Module(source._runtime, source._builtins);
    target._source = source;
    map2.set(source, target);
    copies.push([target, source]);
    modules.add(source);
    return target;
  }
  const derive = alias(this);
  for (const inject of injects) {
    const { alias: alias2, name } = typeof inject === "object" ? inject : { name: inject };
    derive.import(name, alias2 == null ? name : alias2, injectModule);
  }
  for (const module of modules) {
    for (const [name, variable] of module._scope) {
      if (variable._definition === identity) {
        if (module === this && derive._scope.has(name)) continue;
        const importedModule = variable._inputs[0]._module;
        if (importedModule._source) alias(importedModule);
      }
    }
  }
  for (const [target, source] of copies) {
    for (const [name, sourceVariable] of source._scope) {
      const targetVariable = target._scope.get(name);
      if (targetVariable && targetVariable._type !== TYPE_IMPLICIT) continue;
      if (sourceVariable._definition === identity) {
        const sourceInput = sourceVariable._inputs[0];
        const sourceModule = sourceInput._module;
        target.import(sourceInput._name, name, map2.get(sourceModule) || sourceModule);
      } else {
        target.define(name, sourceVariable._inputs.map(variable_name), sourceVariable._definition);
      }
    }
  }
  return derive;
}
function module_resolve(name) {
  let variable = this._scope.get(name), value;
  if (!variable) {
    variable = new Variable(TYPE_IMPLICIT, this);
    if (this._builtins.has(name)) {
      variable.define(name, constant(this._builtins.get(name)));
    } else if (this._runtime._builtin._scope.has(name)) {
      variable.import(name, this._runtime._builtin);
    } else {
      try {
        value = this._runtime._global(name);
      } catch (error) {
        return variable.define(name, rethrow(error));
      }
      if (value === void 0) {
        this._scope.set(variable._name = name, variable);
      } else {
        variable.define(name, constant(value));
      }
    }
  }
  return variable;
}
function module_builtin(name, value) {
  this._builtins.set(name, value);
}
function variable_name(variable) {
  return variable._name;
}
const frame = typeof requestAnimationFrame === "function" ? requestAnimationFrame : typeof setImmediate === "function" ? setImmediate : (f) => setTimeout(f, 0);
function Runtime(builtins, global = window_global) {
  const builtin = this.module();
  Object.defineProperties(this, {
    _dirty: { value: /* @__PURE__ */ new Set() },
    _updates: { value: /* @__PURE__ */ new Set() },
    _precomputes: { value: [], writable: true },
    _computing: { value: null, writable: true },
    _init: { value: null, writable: true },
    _modules: { value: /* @__PURE__ */ new Map() },
    _variables: { value: /* @__PURE__ */ new Set() },
    _disposed: { value: false, writable: true },
    _builtin: { value: builtin },
    _global: { value: global }
  });
  if (builtins) for (const name in builtins) {
    new Variable(TYPE_IMPLICIT, builtin).define(name, [], builtins[name]);
  }
}
Object.defineProperties(Runtime.prototype, {
  _precompute: { value: runtime_precompute, writable: true, configurable: true },
  _compute: { value: runtime_compute, writable: true, configurable: true },
  _computeSoon: { value: runtime_computeSoon, writable: true, configurable: true },
  _computeNow: { value: runtime_computeNow, writable: true, configurable: true },
  dispose: { value: runtime_dispose, writable: true, configurable: true },
  module: { value: runtime_module, writable: true, configurable: true }
});
function runtime_dispose() {
  this._computing = Promise.resolve();
  this._disposed = true;
  this._variables.forEach((v) => {
    v._invalidate();
    v._version = NaN;
  });
}
function runtime_module(define2, observer = noop) {
  let module;
  if (define2 === void 0) {
    if (module = this._init) {
      this._init = null;
      return module;
    }
    return new Module(this);
  }
  module = this._modules.get(define2);
  if (module) return module;
  this._init = module = new Module(this);
  this._modules.set(define2, module);
  try {
    define2(this, observer);
  } finally {
    this._init = null;
  }
  return module;
}
function runtime_precompute(callback) {
  this._precomputes.push(callback);
  this._compute();
}
function runtime_compute() {
  return this._computing || (this._computing = this._computeSoon());
}
function runtime_computeSoon() {
  return new Promise(frame).then(() => this._disposed ? void 0 : this._computeNow());
}
async function runtime_computeNow() {
  let queue2 = [], variables, variable, precomputes = this._precomputes;
  if (precomputes.length) {
    this._precomputes = [];
    for (const callback of precomputes) callback();
    await runtime_defer(3);
  }
  variables = new Set(this._dirty);
  variables.forEach(function(variable2) {
    variable2._inputs.forEach(variables.add, variables);
    const reachable = variable_reachable(variable2);
    if (reachable > variable2._reachable) {
      this._updates.add(variable2);
    } else if (reachable < variable2._reachable) {
      variable2._invalidate();
    }
    variable2._reachable = reachable;
  }, this);
  variables = new Set(this._updates);
  variables.forEach(function(variable2) {
    if (variable2._reachable) {
      variable2._indegree = 0;
      variable2._outputs.forEach(variables.add, variables);
    } else {
      variable2._indegree = NaN;
      variables.delete(variable2);
    }
  });
  this._computing = null;
  this._updates.clear();
  this._dirty.clear();
  variables.forEach(function(variable2) {
    variable2._outputs.forEach(variable_increment);
  });
  do {
    variables.forEach(function(variable2) {
      if (variable2._indegree === 0) {
        queue2.push(variable2);
      }
    });
    while (variable = queue2.pop()) {
      variable_compute(variable);
      variable._outputs.forEach(postqueue);
      variables.delete(variable);
    }
    variables.forEach(function(variable2) {
      if (variable_circular(variable2)) {
        variable_error(variable2, new RuntimeError("circular definition"));
        variable2._outputs.forEach(variable_decrement);
        variables.delete(variable2);
      }
    });
  } while (variables.size);
  function postqueue(variable2) {
    if (--variable2._indegree === 0) {
      queue2.push(variable2);
    }
  }
}
function runtime_defer(depth = 0) {
  let p = Promise.resolve();
  for (let i = 0; i < depth; ++i) p = p.then(() => {
  });
  return p;
}
function variable_circular(variable) {
  const inputs = new Set(variable._inputs);
  for (const i of inputs) {
    if (i === variable) return true;
    i._inputs.forEach(inputs.add, inputs);
  }
  return false;
}
function variable_increment(variable) {
  ++variable._indegree;
}
function variable_decrement(variable) {
  --variable._indegree;
}
function variable_value(variable) {
  return variable._promise.catch(variable._rejector);
}
function variable_invalidator(variable) {
  return new Promise(function(resolve2) {
    variable._invalidate = resolve2;
  });
}
function variable_intersector(invalidation, variable) {
  let node = typeof IntersectionObserver === "function" && variable._observer && variable._observer._node;
  let visible = !node, resolve2 = noop, reject = noop, promise, observer;
  if (node) {
    observer = new IntersectionObserver(([entry]) => (visible = entry.isIntersecting) && (promise = null, resolve2()));
    observer.observe(node);
    invalidation.then(() => (observer.disconnect(), observer = null, reject()));
  }
  return function(value) {
    if (visible) return Promise.resolve(value);
    if (!observer) return Promise.reject();
    if (!promise) promise = new Promise((y, n) => (resolve2 = y, reject = n));
    return promise.then(() => value);
  };
}
function variable_compute(variable) {
  variable._invalidate();
  variable._invalidate = noop;
  variable._pending();
  const value0 = variable._value;
  const version = ++variable._version;
  const inputs = variable._inputs;
  const definition = variable._definition;
  let invalidation = null;
  const promise = variable._promise = variable._promise.then(init, init).then(define2).then(generate);
  function init() {
    return Promise.all(inputs.map(variable_value));
  }
  function define2(inputs2) {
    if (variable._version !== version) throw variable_stale;
    for (let i = 0, n = inputs2.length; i < n; ++i) {
      switch (inputs2[i]) {
        case variable_invalidation: {
          inputs2[i] = invalidation = variable_invalidator(variable);
          break;
        }
        case variable_visibility: {
          if (!invalidation) invalidation = variable_invalidator(variable);
          inputs2[i] = variable_intersector(invalidation, variable);
          break;
        }
        case variable_variable: {
          inputs2[i] = variable;
          break;
        }
      }
    }
    return definition.apply(value0, inputs2);
  }
  function generate(value) {
    if (variable._version !== version) throw variable_stale;
    if (generatorish(value)) {
      (invalidation || variable_invalidator(variable)).then(variable_return(value));
      return variable_generate(variable, version, value);
    }
    return value;
  }
  promise.then((value) => {
    variable._value = value;
    variable._fulfilled(value);
  }, (error) => {
    if (error === variable_stale || variable._version !== version) return;
    variable._value = void 0;
    variable._rejected(error);
  });
}
function variable_generate(variable, version, generator) {
  const runtime = variable._module._runtime;
  let currentValue;
  function compute(onfulfilled) {
    return new Promise((resolve2) => resolve2(generator.next(currentValue))).then(({ done, value }) => {
      return done ? void 0 : Promise.resolve(value).then(onfulfilled);
    });
  }
  function recompute() {
    const promise = compute((value) => {
      if (variable._version !== version) throw variable_stale;
      currentValue = value;
      postcompute(value, promise).then(() => runtime._precompute(recompute));
      variable._fulfilled(value);
      return value;
    });
    promise.catch((error) => {
      if (error === variable_stale || variable._version !== version) return;
      postcompute(void 0, promise);
      variable._rejected(error);
    });
  }
  function postcompute(value, promise) {
    variable._value = value;
    variable._promise = promise;
    variable._outputs.forEach(runtime._updates.add, runtime._updates);
    return runtime._compute();
  }
  return compute((value) => {
    if (variable._version !== version) throw variable_stale;
    currentValue = value;
    runtime._precompute(recompute);
    return value;
  });
}
function variable_error(variable, error) {
  variable._invalidate();
  variable._invalidate = noop;
  variable._pending();
  ++variable._version;
  variable._indegree = NaN;
  (variable._promise = Promise.reject(error)).catch(noop);
  variable._value = void 0;
  variable._rejected(error);
}
function variable_return(generator) {
  return function() {
    generator.return();
  };
}
function variable_reachable(variable) {
  if (variable._observer !== no_observer) return true;
  const outputs = new Set(variable._outputs);
  for (const output of outputs) {
    if (output._observer !== no_observer) return true;
    output._outputs.forEach(outputs.add, outputs);
  }
  return false;
}
function window_global(name) {
  return globalThis[name];
}
function dispatch(node, type, detail) {
  detail = detail || {};
  var document2 = node.ownerDocument, event = document2.defaultView.CustomEvent;
  if (typeof event === "function") {
    event = new event(type, { detail });
  } else {
    event = document2.createEvent("Event");
    event.initEvent(type, false, false);
    event.detail = detail;
  }
  node.dispatchEvent(event);
}
function isarray(value) {
  return Array.isArray(value) || value instanceof Int8Array || value instanceof Int16Array || value instanceof Int32Array || value instanceof Uint8Array || value instanceof Uint8ClampedArray || value instanceof Uint16Array || value instanceof Uint32Array || value instanceof Float32Array || value instanceof Float64Array;
}
function isindex(key) {
  return key === (key | 0) + "";
}
function inspectName(name) {
  const n = document.createElement("span");
  n.className = "observablehq--cellname";
  n.textContent = `${name} = `;
  return n;
}
const symbolToString = Symbol.prototype.toString;
function formatSymbol(symbol) {
  return symbolToString.call(symbol);
}
const { getOwnPropertySymbols, prototype: { hasOwnProperty } } = Object;
const { toStringTag } = Symbol;
const FORBIDDEN = {};
const symbolsof = getOwnPropertySymbols;
function isown(object, key) {
  return hasOwnProperty.call(object, key);
}
function tagof(object) {
  return object[toStringTag] || object.constructor && object.constructor.name || "Object";
}
function valueof$1(object, key) {
  try {
    const value = object[key];
    if (value) value.constructor;
    return value;
  } catch (ignore) {
    return FORBIDDEN;
  }
}
const SYMBOLS = [
  { symbol: "@@__IMMUTABLE_INDEXED__@@", name: "Indexed", modifier: true },
  { symbol: "@@__IMMUTABLE_KEYED__@@", name: "Keyed", modifier: true },
  { symbol: "@@__IMMUTABLE_LIST__@@", name: "List", arrayish: true },
  { symbol: "@@__IMMUTABLE_MAP__@@", name: "Map" },
  {
    symbol: "@@__IMMUTABLE_ORDERED__@@",
    name: "Ordered",
    modifier: true,
    prefix: true
  },
  { symbol: "@@__IMMUTABLE_RECORD__@@", name: "Record" },
  {
    symbol: "@@__IMMUTABLE_SET__@@",
    name: "Set",
    arrayish: true,
    setish: true
  },
  { symbol: "@@__IMMUTABLE_STACK__@@", name: "Stack", arrayish: true }
];
function immutableName(obj) {
  try {
    let symbols = SYMBOLS.filter(({ symbol }) => obj[symbol] === true);
    if (!symbols.length) return;
    const name = symbols.find((s) => !s.modifier);
    const prefix = name.name === "Map" && symbols.find((s) => s.modifier && s.prefix);
    const arrayish = symbols.some((s) => s.arrayish);
    const setish = symbols.some((s) => s.setish);
    return {
      name: `${prefix ? prefix.name : ""}${name.name}`,
      symbols,
      arrayish: arrayish && !setish,
      setish
    };
  } catch (e) {
    return null;
  }
}
const { getPrototypeOf, getOwnPropertyDescriptors } = Object;
const objectPrototype = getPrototypeOf({});
function inspectExpanded(object, _2, name, proto) {
  let arrayish = isarray(object);
  let tag, fields, next, n;
  if (object instanceof Map) {
    if (object instanceof object.constructor) {
      tag = `Map(${object.size})`;
      fields = iterateMap$1;
    } else {
      tag = "Map()";
      fields = iterateObject$1;
    }
  } else if (object instanceof Set) {
    if (object instanceof object.constructor) {
      tag = `Set(${object.size})`;
      fields = iterateSet$1;
    } else {
      tag = "Set()";
      fields = iterateObject$1;
    }
  } else if (arrayish) {
    tag = `${object.constructor.name}(${object.length})`;
    fields = iterateArray$1;
  } else if (n = immutableName(object)) {
    tag = `Immutable.${n.name}${n.name === "Record" ? "" : `(${object.size})`}`;
    arrayish = n.arrayish;
    fields = n.arrayish ? iterateImArray$1 : n.setish ? iterateImSet$1 : iterateImObject$1;
  } else if (proto) {
    tag = tagof(object);
    fields = iterateProto;
  } else {
    tag = tagof(object);
    fields = iterateObject$1;
  }
  const span = document.createElement("span");
  span.className = "observablehq--expanded";
  if (name) {
    span.appendChild(inspectName(name));
  }
  const a = span.appendChild(document.createElement("a"));
  a.innerHTML = `<svg width=8 height=8 class='observablehq--caret'>
    <path d='M4 7L0 1h8z' fill='currentColor' />
  </svg>`;
  a.appendChild(document.createTextNode(`${tag}${arrayish ? " [" : " {"}`));
  a.addEventListener("mouseup", function(event) {
    event.stopPropagation();
    replace(span, inspectCollapsed(object, null, name, proto));
  });
  fields = fields(object);
  for (let i = 0; !(next = fields.next()).done && i < 20; ++i) {
    span.appendChild(next.value);
  }
  if (!next.done) {
    const a2 = span.appendChild(document.createElement("a"));
    a2.className = "observablehq--field";
    a2.style.display = "block";
    a2.appendChild(document.createTextNode(`  … more`));
    a2.addEventListener("mouseup", function(event) {
      event.stopPropagation();
      span.insertBefore(next.value, span.lastChild.previousSibling);
      for (let i = 0; !(next = fields.next()).done && i < 19; ++i) {
        span.insertBefore(next.value, span.lastChild.previousSibling);
      }
      if (next.done) span.removeChild(span.lastChild.previousSibling);
      dispatch(span, "load");
    });
  }
  span.appendChild(document.createTextNode(arrayish ? "]" : "}"));
  return span;
}
function* iterateMap$1(map2) {
  for (const [key, value] of map2) {
    yield formatMapField$1(key, value);
  }
  yield* iterateObject$1(map2);
}
function* iterateSet$1(set) {
  for (const value of set) {
    yield formatSetField(value);
  }
  yield* iterateObject$1(set);
}
function* iterateImSet$1(set) {
  for (const value of set) {
    yield formatSetField(value);
  }
}
function* iterateArray$1(array) {
  for (let i = 0, n = array.length; i < n; ++i) {
    if (i in array) {
      yield formatField$1(i, valueof$1(array, i), "observablehq--index");
    }
  }
  for (const key in array) {
    if (!isindex(key) && isown(array, key)) {
      yield formatField$1(key, valueof$1(array, key), "observablehq--key");
    }
  }
  for (const symbol of symbolsof(array)) {
    yield formatField$1(
      formatSymbol(symbol),
      valueof$1(array, symbol),
      "observablehq--symbol"
    );
  }
}
function* iterateImArray$1(array) {
  let i1 = 0;
  for (const n = array.size; i1 < n; ++i1) {
    yield formatField$1(i1, array.get(i1), true);
  }
}
function* iterateProto(object) {
  for (const key in getOwnPropertyDescriptors(object)) {
    yield formatField$1(key, valueof$1(object, key), "observablehq--key");
  }
  for (const symbol of symbolsof(object)) {
    yield formatField$1(
      formatSymbol(symbol),
      valueof$1(object, symbol),
      "observablehq--symbol"
    );
  }
  const proto = getPrototypeOf(object);
  if (proto && proto !== objectPrototype) {
    yield formatPrototype(proto);
  }
}
function* iterateObject$1(object) {
  for (const key in object) {
    if (isown(object, key)) {
      yield formatField$1(key, valueof$1(object, key), "observablehq--key");
    }
  }
  for (const symbol of symbolsof(object)) {
    yield formatField$1(
      formatSymbol(symbol),
      valueof$1(object, symbol),
      "observablehq--symbol"
    );
  }
  const proto = getPrototypeOf(object);
  if (proto && proto !== objectPrototype) {
    yield formatPrototype(proto);
  }
}
function* iterateImObject$1(object) {
  for (const [key, value] of object) {
    yield formatField$1(key, value, "observablehq--key");
  }
}
function formatPrototype(value) {
  const item = document.createElement("div");
  const span = item.appendChild(document.createElement("span"));
  item.className = "observablehq--field";
  span.className = "observablehq--prototype-key";
  span.textContent = `  <prototype>`;
  item.appendChild(document.createTextNode(": "));
  item.appendChild(inspect$1(value, void 0, void 0, void 0, true));
  return item;
}
function formatField$1(key, value, className) {
  const item = document.createElement("div");
  const span = item.appendChild(document.createElement("span"));
  item.className = "observablehq--field";
  span.className = className;
  span.textContent = `  ${key}`;
  item.appendChild(document.createTextNode(": "));
  item.appendChild(inspect$1(value));
  return item;
}
function formatMapField$1(key, value) {
  const item = document.createElement("div");
  item.className = "observablehq--field";
  item.appendChild(document.createTextNode("  "));
  item.appendChild(inspect$1(key));
  item.appendChild(document.createTextNode(" => "));
  item.appendChild(inspect$1(value));
  return item;
}
function formatSetField(value) {
  const item = document.createElement("div");
  item.className = "observablehq--field";
  item.appendChild(document.createTextNode("  "));
  item.appendChild(inspect$1(value));
  return item;
}
function hasSelection(elem) {
  const sel = window.getSelection();
  return sel.type === "Range" && (sel.containsNode(elem, true) || elem.contains(sel.anchorNode) || elem.contains(sel.focusNode));
}
function inspectCollapsed(object, shallow, name, proto) {
  let arrayish = isarray(object);
  let tag, fields, next, n;
  if (object instanceof Map) {
    if (object instanceof object.constructor) {
      tag = `Map(${object.size})`;
      fields = iterateMap;
    } else {
      tag = "Map()";
      fields = iterateObject;
    }
  } else if (object instanceof Set) {
    if (object instanceof object.constructor) {
      tag = `Set(${object.size})`;
      fields = iterateSet;
    } else {
      tag = "Set()";
      fields = iterateObject;
    }
  } else if (arrayish) {
    tag = `${object.constructor.name}(${object.length})`;
    fields = iterateArray;
  } else if (n = immutableName(object)) {
    tag = `Immutable.${n.name}${n.name === "Record" ? "" : `(${object.size})`}`;
    arrayish = n.arrayish;
    fields = n.arrayish ? iterateImArray : n.setish ? iterateImSet : iterateImObject;
  } else {
    tag = tagof(object);
    fields = iterateObject;
  }
  if (shallow) {
    const span2 = document.createElement("span");
    span2.className = "observablehq--shallow";
    if (name) {
      span2.appendChild(inspectName(name));
    }
    span2.appendChild(document.createTextNode(tag));
    span2.addEventListener("mouseup", function(event) {
      if (hasSelection(span2)) return;
      event.stopPropagation();
      replace(span2, inspectCollapsed(object));
    });
    return span2;
  }
  const span = document.createElement("span");
  span.className = "observablehq--collapsed";
  if (name) {
    span.appendChild(inspectName(name));
  }
  const a = span.appendChild(document.createElement("a"));
  a.innerHTML = `<svg width=8 height=8 class='observablehq--caret'>
    <path d='M7 4L1 8V0z' fill='currentColor' />
  </svg>`;
  a.appendChild(document.createTextNode(`${tag}${arrayish ? " [" : " {"}`));
  span.addEventListener("mouseup", function(event) {
    if (hasSelection(span)) return;
    event.stopPropagation();
    replace(span, inspectExpanded(object, null, name, proto));
  }, true);
  fields = fields(object);
  for (let i = 0; !(next = fields.next()).done && i < 20; ++i) {
    if (i > 0) span.appendChild(document.createTextNode(", "));
    span.appendChild(next.value);
  }
  if (!next.done) span.appendChild(document.createTextNode(", …"));
  span.appendChild(document.createTextNode(arrayish ? "]" : "}"));
  return span;
}
function* iterateMap(map2) {
  for (const [key, value] of map2) {
    yield formatMapField(key, value);
  }
  yield* iterateObject(map2);
}
function* iterateSet(set) {
  for (const value of set) {
    yield inspect$1(value, true);
  }
  yield* iterateObject(set);
}
function* iterateImSet(set) {
  for (const value of set) {
    yield inspect$1(value, true);
  }
}
function* iterateImArray(array) {
  let i0 = -1, i1 = 0;
  for (const n = array.size; i1 < n; ++i1) {
    if (i1 > i0 + 1) yield formatEmpty(i1 - i0 - 1);
    yield inspect$1(array.get(i1), true);
    i0 = i1;
  }
  if (i1 > i0 + 1) yield formatEmpty(i1 - i0 - 1);
}
function* iterateArray(array) {
  let i0 = -1, i1 = 0;
  for (const n = array.length; i1 < n; ++i1) {
    if (i1 in array) {
      if (i1 > i0 + 1) yield formatEmpty(i1 - i0 - 1);
      yield inspect$1(valueof$1(array, i1), true);
      i0 = i1;
    }
  }
  if (i1 > i0 + 1) yield formatEmpty(i1 - i0 - 1);
  for (const key in array) {
    if (!isindex(key) && isown(array, key)) {
      yield formatField(key, valueof$1(array, key), "observablehq--key");
    }
  }
  for (const symbol of symbolsof(array)) {
    yield formatField(formatSymbol(symbol), valueof$1(array, symbol), "observablehq--symbol");
  }
}
function* iterateObject(object) {
  for (const key in object) {
    if (isown(object, key)) {
      yield formatField(key, valueof$1(object, key), "observablehq--key");
    }
  }
  for (const symbol of symbolsof(object)) {
    yield formatField(formatSymbol(symbol), valueof$1(object, symbol), "observablehq--symbol");
  }
}
function* iterateImObject(object) {
  for (const [key, value] of object) {
    yield formatField(key, value, "observablehq--key");
  }
}
function formatEmpty(e) {
  const span = document.createElement("span");
  span.className = "observablehq--empty";
  span.textContent = e === 1 ? "empty" : `empty × ${e}`;
  return span;
}
function formatField(key, value, className) {
  const fragment = document.createDocumentFragment();
  const span = fragment.appendChild(document.createElement("span"));
  span.className = className;
  span.textContent = key;
  fragment.appendChild(document.createTextNode(": "));
  fragment.appendChild(inspect$1(value, true));
  return fragment;
}
function formatMapField(key, value) {
  const fragment = document.createDocumentFragment();
  fragment.appendChild(inspect$1(key, true));
  fragment.appendChild(document.createTextNode(" => "));
  fragment.appendChild(inspect$1(value, true));
  return fragment;
}
function format(date, fallback) {
  if (!(date instanceof Date)) date = /* @__PURE__ */ new Date(+date);
  if (isNaN(date)) return typeof fallback === "function" ? fallback(date) : fallback;
  const hours = date.getUTCHours();
  const minutes = date.getUTCMinutes();
  const seconds = date.getUTCSeconds();
  const milliseconds = date.getUTCMilliseconds();
  return `${formatYear(date.getUTCFullYear())}-${pad(date.getUTCMonth() + 1, 2)}-${pad(date.getUTCDate(), 2)}${hours || minutes || seconds || milliseconds ? `T${pad(hours, 2)}:${pad(minutes, 2)}${seconds || milliseconds ? `:${pad(seconds, 2)}${milliseconds ? `.${pad(milliseconds, 3)}` : ``}` : ``}Z` : ``}`;
}
function formatYear(year) {
  return year < 0 ? `-${pad(-year, 6)}` : year > 9999 ? `+${pad(year, 6)}` : pad(year, 4);
}
function pad(value, width2) {
  return `${value}`.padStart(width2, "0");
}
function formatDate(date) {
  return format(date, "Invalid Date");
}
var errorToString = Error.prototype.toString;
function formatError(value) {
  return value.stack || errorToString.call(value);
}
var regExpToString = RegExp.prototype.toString;
function formatRegExp(value) {
  return regExpToString.call(value);
}
const NEWLINE_LIMIT = 20;
function formatString(string, shallow, expanded, name) {
  if (shallow === false) {
    if (count$1(string, /["\n]/g) <= count$1(string, /`|\${/g)) {
      const span3 = document.createElement("span");
      if (name) span3.appendChild(inspectName(name));
      const textValue3 = span3.appendChild(document.createElement("span"));
      textValue3.className = "observablehq--string";
      textValue3.textContent = JSON.stringify(string);
      return span3;
    }
    const lines = string.split("\n");
    if (lines.length > NEWLINE_LIMIT && !expanded) {
      const div = document.createElement("div");
      if (name) div.appendChild(inspectName(name));
      const textValue3 = div.appendChild(document.createElement("span"));
      textValue3.className = "observablehq--string";
      textValue3.textContent = "`" + templatify(lines.slice(0, NEWLINE_LIMIT).join("\n"));
      const splitter = div.appendChild(document.createElement("span"));
      const truncatedCount = lines.length - NEWLINE_LIMIT;
      splitter.textContent = `Show ${truncatedCount} truncated line${truncatedCount > 1 ? "s" : ""}`;
      splitter.className = "observablehq--string-expand";
      splitter.addEventListener("mouseup", function(event) {
        event.stopPropagation();
        replace(div, inspect$1(string, shallow, true, name));
      });
      return div;
    }
    const span2 = document.createElement("span");
    if (name) span2.appendChild(inspectName(name));
    const textValue2 = span2.appendChild(document.createElement("span"));
    textValue2.className = `observablehq--string${expanded ? " observablehq--expanded" : ""}`;
    textValue2.textContent = "`" + templatify(string) + "`";
    return span2;
  }
  const span = document.createElement("span");
  if (name) span.appendChild(inspectName(name));
  const textValue = span.appendChild(document.createElement("span"));
  textValue.className = "observablehq--string";
  textValue.textContent = JSON.stringify(string.length > 100 ? `${string.slice(0, 50)}…${string.slice(-49)}` : string);
  return span;
}
function templatify(string) {
  return string.replace(/[\\`\x00-\x09\x0b-\x19]|\${/g, templatifyChar);
}
function templatifyChar(char) {
  var code = char.charCodeAt(0);
  switch (code) {
    case 8:
      return "\\b";
    case 9:
      return "\\t";
    case 11:
      return "\\v";
    case 12:
      return "\\f";
    case 13:
      return "\\r";
  }
  return code < 16 ? "\\x0" + code.toString(16) : code < 32 ? "\\x" + code.toString(16) : "\\" + char;
}
function count$1(string, re) {
  var n = 0;
  while (re.exec(string)) ++n;
  return n;
}
var toString$1 = Function.prototype.toString, TYPE_ASYNC = { prefix: "async ƒ" }, TYPE_ASYNC_GENERATOR = { prefix: "async ƒ*" }, TYPE_CLASS = { prefix: "class" }, TYPE_FUNCTION = { prefix: "ƒ" }, TYPE_GENERATOR = { prefix: "ƒ*" };
function inspectFunction(f, name) {
  var type, m, t = toString$1.call(f);
  switch (f.constructor && f.constructor.name) {
    case "AsyncFunction":
      type = TYPE_ASYNC;
      break;
    case "AsyncGeneratorFunction":
      type = TYPE_ASYNC_GENERATOR;
      break;
    case "GeneratorFunction":
      type = TYPE_GENERATOR;
      break;
    default:
      type = /^class\b/.test(t) ? TYPE_CLASS : TYPE_FUNCTION;
      break;
  }
  if (type === TYPE_CLASS) {
    return formatFunction(type, "", name);
  }
  if (m = /^(?:async\s*)?(\w+)\s*=>/.exec(t)) {
    return formatFunction(type, "(" + m[1] + ")", name);
  }
  if (m = /^(?:async\s*)?\(\s*(\w+(?:\s*,\s*\w+)*)?\s*\)/.exec(t)) {
    return formatFunction(type, m[1] ? "(" + m[1].replace(/\s*,\s*/g, ", ") + ")" : "()", name);
  }
  if (m = /^(?:async\s*)?function(?:\s*\*)?(?:\s*\w+)?\s*\(\s*(\w+(?:\s*,\s*\w+)*)?\s*\)/.exec(t)) {
    return formatFunction(type, m[1] ? "(" + m[1].replace(/\s*,\s*/g, ", ") + ")" : "()", name);
  }
  return formatFunction(type, "(…)", name);
}
function formatFunction(type, args, cellname) {
  var span = document.createElement("span");
  span.className = "observablehq--function";
  if (cellname) {
    span.appendChild(inspectName(cellname));
  }
  var spanType = span.appendChild(document.createElement("span"));
  spanType.className = "observablehq--keyword";
  spanType.textContent = type.prefix;
  span.appendChild(document.createTextNode(args));
  return span;
}
const { prototype: { toString } } = Object;
function inspect$1(value, shallow, expand, name, proto) {
  let type = typeof value;
  switch (type) {
    case "boolean":
    case "undefined": {
      value += "";
      break;
    }
    case "number": {
      value = value === 0 && 1 / value < 0 ? "-0" : value + "";
      break;
    }
    case "bigint": {
      value = value + "n";
      break;
    }
    case "symbol": {
      value = formatSymbol(value);
      break;
    }
    case "function": {
      return inspectFunction(value, name);
    }
    case "string": {
      return formatString(value, shallow, expand, name);
    }
    default: {
      if (value === null) {
        type = null, value = "null";
        break;
      }
      if (value instanceof Date) {
        type = "date", value = formatDate(value);
        break;
      }
      if (value === FORBIDDEN) {
        type = "forbidden", value = "[forbidden]";
        break;
      }
      switch (toString.call(value)) {
        case "[object RegExp]": {
          type = "regexp", value = formatRegExp(value);
          break;
        }
        case "[object Error]":
        // https://github.com/lodash/lodash/blob/master/isError.js#L26
        case "[object DOMException]": {
          type = "error", value = formatError(value);
          break;
        }
        default:
          return (expand ? inspectExpanded : inspectCollapsed)(value, shallow, name, proto);
      }
      break;
    }
  }
  const span = document.createElement("span");
  if (name) span.appendChild(inspectName(name));
  const n = span.appendChild(document.createElement("span"));
  n.className = `observablehq--${type}`;
  n.textContent = value;
  return span;
}
function replace(spanOld, spanNew) {
  if (spanOld.classList.contains("observablehq--inspect")) spanNew.classList.add("observablehq--inspect");
  spanOld.parentNode.replaceChild(spanNew, spanOld);
  dispatch(spanNew, "load");
}
const LOCATION_MATCH = /\s+\(\d+:\d+\)$/m;
class Inspector {
  constructor(node) {
    if (!node) throw new Error("invalid node");
    this._node = node;
    node.classList.add("observablehq");
  }
  pending() {
    const { _node } = this;
    _node.classList.remove("observablehq--error");
    _node.classList.add("observablehq--running");
  }
  fulfilled(value, name) {
    const { _node } = this;
    if (!isnode(value) || value.parentNode && value.parentNode !== _node) {
      value = inspect$1(value, false, _node.firstChild && _node.firstChild.classList && _node.firstChild.classList.contains("observablehq--expanded"), name);
      value.classList.add("observablehq--inspect");
    }
    _node.classList.remove("observablehq--running", "observablehq--error");
    if (_node.firstChild !== value) {
      if (_node.firstChild) {
        while (_node.lastChild !== _node.firstChild) _node.removeChild(_node.lastChild);
        _node.replaceChild(value, _node.firstChild);
      } else {
        _node.appendChild(value);
      }
    }
    dispatch(_node, "update");
  }
  rejected(error, name) {
    const { _node } = this;
    _node.classList.remove("observablehq--running");
    _node.classList.add("observablehq--error");
    while (_node.lastChild) _node.removeChild(_node.lastChild);
    var div = document.createElement("div");
    div.className = "observablehq--inspect";
    if (name) div.appendChild(inspectName(name));
    div.appendChild(document.createTextNode((error + "").replace(LOCATION_MATCH, "")));
    _node.appendChild(div);
    dispatch(_node, "error", { error });
  }
}
Inspector.into = function(container) {
  if (typeof container === "string") {
    container = document.querySelector(container);
    if (container == null) throw new Error("container not found");
  }
  return function() {
    return new Inspector(container.appendChild(document.createElement("div")));
  };
};
function isnode(value) {
  return (value instanceof Element || value instanceof Text) && value instanceof value.constructor;
}
function inspect(value, expanded) {
  const node = document.createElement("div");
  new Inspector(node).fulfilled(value);
  if (expanded) {
    for (const path of expanded) {
      let child = node;
      for (const i of path)
        child = child?.childNodes[i];
      child?.dispatchEvent(new Event("mouseup"));
    }
  }
  return node;
}
function inspectError(value) {
  const node = document.createElement("div");
  new Inspector(node).rejected(value);
  return node;
}
function getExpanded(node) {
  if (!isInspector(node))
    return;
  const expanded = node.querySelectorAll(".observablehq--expanded");
  if (expanded.length)
    return Array.from(expanded, (e) => getNodePath(node, e));
}
function isElement(node) {
  return node.nodeType === 1;
}
function isInspector(node) {
  return isElement(node) && node.classList.contains("observablehq");
}
function getNodePath(node, descendant) {
  const path = [];
  while (descendant !== node) {
    path.push(getChildIndex(descendant));
    descendant = descendant.parentNode;
  }
  return path.reverse();
}
function getChildIndex(node) {
  return Array.prototype.indexOf.call(node.parentNode.childNodes, node);
}
const SRC_SELECTOR = [
  "audio source[src]",
  // audio
  "audio[src]",
  // audio
  "img[src]",
  // images
  "picture source[src]",
  // images
  "video source[src]",
  // videos
  "video[src]"
  // videos
].join();
const SRCSET_SELECTOR = [
  "img[srcset]",
  // images
  "picture source[srcset]"
  // images
].join();
const HREF_SELECTOR = [
  "a[href][download]",
  // download links
  "link[href]"
  // stylesheets
].join();
const ASSET_ATTRIBUTES = [
  [SRC_SELECTOR, "src"],
  [SRCSET_SELECTOR, "srcset"],
  [HREF_SELECTOR, "href"]
];
function mapAssets(root2, assets) {
  const resolve2 = (s) => assets.get(asImportPath(s)) ?? s;
  for (const [selector, src] of ASSET_ATTRIBUTES) {
    for (const element of root2.querySelectorAll(selector)) {
      if (isRelExternal(element))
        continue;
      const source = decodeURI(element.getAttribute(src));
      if (src === "srcset")
        element.setAttribute(src, resolveSrcset(source, resolve2));
      else
        element.setAttribute(src, resolve2(source));
    }
  }
}
function isRelExternal(a) {
  return /(?:^|\s)external(?:\s|$)/i.test(a.getAttribute("rel") ?? "");
}
function asPath(source) {
  const i = source.indexOf("?");
  const j = source.indexOf("#");
  const k = i >= 0 && j >= 0 ? Math.min(i, j) : i >= 0 ? i : j;
  return k >= 0 ? source.slice(0, k) : source;
}
function asImportPath(source) {
  const path = asPath(source);
  return isImportPath(path) ? path : `./${path}`;
}
function isImportPath(specifier) {
  return ["./", "../", "/"].some((prefix) => specifier.startsWith(prefix));
}
function resolveSrcset(srcset, resolve2) {
  return srcset.trim().split(/\s*,\s*/).filter((src) => src).map((src) => {
    const parts = src.split(/\s+/);
    const path = resolve2(parts[0]);
    if (path)
      parts[0] = encodeURI(path);
    return parts.join(" ");
  }).join(", ");
}
function display(state, value) {
  const { root: root2, expanded } = state;
  const node = isDisplayable(value, root2) ? value : inspect(value, expanded[root2.childNodes.length]);
  displayNode(state, node);
}
function displayNode(state, node) {
  if (node.nodeType === 11) {
    let child;
    while (child = node.firstChild) {
      state.root.appendChild(child);
    }
  } else {
    state.root.appendChild(node);
  }
}
function displayError(state, value) {
  displayNode(state, inspectError(value));
}
function isDisplayable(value, root2) {
  return (value instanceof Element || value instanceof Text) && value instanceof value.constructor && (!value.parentNode || root2.contains(value));
}
function clear(state) {
  state.autoclear = false;
  state.expanded = Array.from(state.root.childNodes, getExpanded);
  while (state.root.lastChild)
    state.root.lastChild.remove();
}
function observe$1(state, { autodisplay, assets }) {
  return {
    _error: false,
    _node: state.root,
    // _node for visibility promise
    pending() {
      if (this._error) {
        this._error = false;
        clear(state);
      }
    },
    fulfilled(value) {
      if (autodisplay) {
        if (assets && value instanceof Element)
          mapAssets(value, assets);
        clear(state);
        display(state, value);
      } else if (state.autoclear) {
        clear(state);
      }
    },
    rejected(error) {
      console.error(error);
      this._error = true;
      clear(state);
      displayError(state, error);
    }
  };
}
async function* observe(initialize) {
  let resolve2;
  let value;
  let stale = false;
  const dispose = initialize((x) => {
    value = x;
    if (resolve2) {
      resolve2(x);
      resolve2 = void 0;
    } else {
      stale = true;
    }
    return x;
  });
  if (dispose != null && typeof dispose !== "function") {
    throw new Error(typeof dispose === "object" && "then" in dispose && typeof dispose.then === "function" ? "async initializers are not supported" : "initializer returned something, but not a dispose function");
  }
  try {
    while (true) {
      yield stale ? (stale = false, value) : new Promise((_2) => resolve2 = _2);
    }
  } finally {
    if (dispose != null) {
      dispose();
    }
  }
}
function input(element) {
  return observe((change) => {
    const event = eventof(element);
    const value = valueof(element);
    const inputted = () => change(valueof(element));
    element.addEventListener(event, inputted);
    if (value !== void 0)
      change(value);
    return () => element.removeEventListener(event, inputted);
  });
}
function valueof(element) {
  const input2 = element;
  const select = element;
  if ("type" in element) {
    switch (element.type) {
      case "range":
      case "number":
        return input2.valueAsNumber;
      case "date":
        return input2.valueAsDate;
      case "checkbox":
        return input2.checked;
      case "file":
        return input2.multiple ? input2.files : input2.files[0];
      case "select-multiple":
        return Array.from(select.selectedOptions, (o) => o.value);
    }
  }
  return input2.value;
}
function eventof(element) {
  if ("type" in element) {
    switch (element.type) {
      case "button":
      case "submit":
      case "checkbox":
        return "click";
      case "file":
        return "change";
    }
  }
  return "input";
}
async function* now() {
  while (true) {
    yield Date.now();
  }
}
async function* queue(initialize) {
  let resolve2;
  const values = [];
  const dispose = initialize((x) => {
    values.push(x);
    if (resolve2) {
      resolve2(values.shift());
      resolve2 = void 0;
    }
    return x;
  });
  if (dispose != null && typeof dispose !== "function") {
    throw new Error(typeof dispose === "object" && "then" in dispose && typeof dispose.then === "function" ? "async initializers are not supported" : "initializer returned something, but not a dispose function");
  }
  try {
    while (true) {
      yield values.length ? values.shift() : new Promise((_2) => resolve2 = _2);
    }
  } finally {
    if (dispose != null) {
      dispose();
    }
  }
}
function width(target, options) {
  return observe((notify) => {
    let width2;
    const observer = new ResizeObserver(([entry]) => {
      const w = entry.contentRect.width;
      if (w !== width2)
        notify(width2 = w);
    });
    observer.observe(target, options);
    return () => observer.disconnect();
  });
}
const Generators = /* @__PURE__ */ Object.freeze(/* @__PURE__ */ Object.defineProperty({
  __proto__: null,
  input,
  now,
  observe,
  queue,
  width
}, Symbol.toStringTag, { value: "Module" }));
function Mutable(value) {
  let change;
  return Object.defineProperty(observe((_2) => {
    change = _2;
    if (value !== void 0)
      change(value);
  }), "value", {
    get: () => value,
    set: (x) => (value = x, void change?.(value))
  });
}
function Mutator(value) {
  const mutable = Mutable(value);
  return [
    mutable,
    {
      get value() {
        return mutable.value;
      },
      set value(v) {
        mutable.value = v;
      }
    }
  ];
}
function define$1(main2, state, definition, observer = observe$1) {
  const { id, body, inputs = [], outputs = [], output, autodisplay, autoview, automutable } = definition;
  const variables = state.variables;
  const v = main2.variable(observer(state, definition), { shadow: {} });
  const vid = output ?? (outputs.length ? `cell ${id}` : null);
  state.autoclear = true;
  if (inputs.includes("display") || inputs.includes("view")) {
    let displayVersion = -1;
    const vd = new v.constructor(2, v._module);
    vd.define(inputs.filter((i) => i !== "display" && i !== "view"), () => {
      const version = v._version;
      return (value) => {
        if (version < displayVersion)
          throw new Error("stale display");
        else if (state.variables[0] !== v)
          throw new Error("stale display");
        else if (version > displayVersion)
          clear(state);
        displayVersion = version;
        display(state, value);
        return value;
      };
    });
    v._shadow.set("display", vd);
    if (inputs.includes("view")) {
      const vv = new v.constructor(2, v._module, null, { shadow: {} });
      vv._shadow.set("display", vd);
      vv.define(["display"], (display2) => (value) => input(display2(value)));
      v._shadow.set("view", vv);
    }
  } else if (!autodisplay) {
    clear(state);
  }
  variables.push(v.define(vid, inputs, body));
  if (output != null) {
    if (autoview) {
      const o = unprefix(output, "viewof$");
      variables.push(main2.define(o, [output], input));
    } else if (automutable) {
      const o = unprefix(output, "mutable ");
      const x = `cell ${id}`;
      v.define(o, [x], ([mutable]) => mutable);
      variables.push(
        main2.define(output, inputs, body),
        // initial value
        main2.define(x, [output], Mutator),
        main2.define(`mutable$${o}`, [x], ([, mutator]) => mutator)
      );
    }
  } else {
    for (const o of outputs) {
      variables.push(main2.variable(true).define(o, [vid], (exports$1) => exports$1[o]));
    }
  }
}
function unprefix(name, prefix) {
  if (!name.startsWith(prefix))
    throw new Error(`expected ${prefix}: ${name}`);
  return name.slice(prefix.length);
}
const scriptRel = "modulepreload";
const assetsURL = function(dep) {
  return "/opa/benchmarks/" + dep;
};
const seen = {};
const __vitePreload = function preload(baseModule, deps, importerUrl) {
  let promise = Promise.resolve();
  if (deps && deps.length > 0) {
    let allSettled = function(promises$2) {
      return Promise.all(promises$2.map((p) => Promise.resolve(p).then((value$1) => ({
        status: "fulfilled",
        value: value$1
      }), (reason) => ({
        status: "rejected",
        reason
      }))));
    };
    document.getElementsByTagName("link");
    const cspNonceMeta = document.querySelector("meta[property=csp-nonce]");
    const cspNonce = cspNonceMeta?.nonce || cspNonceMeta?.getAttribute("nonce");
    promise = allSettled(deps.map((dep) => {
      dep = assetsURL(dep);
      if (dep in seen) return;
      seen[dep] = true;
      const isCss = dep.endsWith(".css");
      const cssSelector = isCss ? '[rel="stylesheet"]' : "";
      if (document.querySelector(`link[href="${dep}"]${cssSelector}`)) return;
      const link = document.createElement("link");
      link.rel = isCss ? "stylesheet" : scriptRel;
      if (!isCss) link.as = "script";
      link.crossOrigin = "";
      link.href = dep;
      if (cspNonce) link.setAttribute("nonce", cspNonce);
      document.head.appendChild(link);
      if (isCss) return new Promise((res, rej) => {
        link.addEventListener("load", res);
        link.addEventListener("error", () => rej(/* @__PURE__ */ new Error(`Unable to preload CSS for ${dep}`)));
      });
    }));
  }
  function handlePreloadError(err$2) {
    const e$1 = new Event("vite:preloadError", { cancelable: true });
    e$1.payload = err$2;
    window.dispatchEvent(e$1);
    if (!e$1.defaultPrevented) throw err$2;
  }
  return promise.then((res) => {
    for (const item of res || []) {
      if (item.status !== "rejected") continue;
      handlePreloadError(item.reason);
    }
    return baseModule().catch(handlePreloadError);
  });
};
const files = /* @__PURE__ */ new Map();
const FileAttachment = (name, base = document.baseURI) => {
  const href = new URL(name, base).href;
  let file = files.get(href);
  if (!file) {
    file = new FileAttachmentImpl(href, name.split("/").pop());
    files.set(href, file);
  }
  return file;
};
async function fetchFile(file) {
  const response = await fetch(file.href);
  if (!response.ok)
    throw new Error(`Unable to load file: ${file.name}`);
  return response;
}
class AbstractFile {
  constructor(name, mimeType = guessMimeType(name), lastModified, size) {
    Object.defineProperty(this, "name", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "mimeType", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "lastModified", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "size", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperties(this, {
      name: { value: `${name}`, enumerable: true },
      mimeType: { value: `${mimeType}`, enumerable: true },
      lastModified: { value: lastModified === void 0 ? void 0 : +lastModified, enumerable: true },
      // prettier-ignore
      size: { value: size === void 0 ? void 0 : +size, enumerable: true }
    });
  }
  async url() {
    return this.href;
  }
  async blob() {
    return (await fetchFile(this)).blob();
  }
  async arrayBuffer() {
    return (await fetchFile(this)).arrayBuffer();
  }
  async text(encoding) {
    return encoding === void 0 ? (await fetchFile(this)).text() : new TextDecoder(encoding).decode(await this.arrayBuffer());
  }
  async json() {
    return (await fetchFile(this)).json();
  }
  async stream() {
    return (await fetchFile(this)).body;
  }
  async dsv({ delimiter = ",", array = false, typed = false } = {}) {
    const [text2, d32] = await Promise.all([this.text(), __vitePreload(() => import("https://cdn.jsdelivr.net/npm/d3-dsv/+esm"), true ? [] : void 0)]);
    const format2 = d32.dsvFormat(delimiter);
    const parse = array ? format2.parseRows : format2.parse;
    return parse(text2, typed && d32.autoType);
  }
  async csv(options) {
    return this.dsv({ ...options, delimiter: "," });
  }
  async tsv(options) {
    return this.dsv({ ...options, delimiter: "	" });
  }
  async image(props) {
    const url = await this.url();
    return new Promise((resolve2, reject) => {
      const i = new Image();
      if (new URL(url, document.baseURI).origin !== location.origin)
        i.crossOrigin = "anonymous";
      Object.assign(i, props);
      i.onload = () => resolve2(i);
      i.onerror = () => reject(new Error(`Unable to load file: ${this.name}`));
      i.src = url;
    });
  }
  async arrow() {
    const [Arrow2, response] = await Promise.all([__vitePreload(() => import("https://cdn.jsdelivr.net/npm/apache-arrow@17.0.0/+esm"), true ? [] : void 0), fetchFile(this)]);
    return Arrow2.tableFromIPC(response);
  }
  async arquero(options) {
    let request;
    let from;
    switch (this.mimeType) {
      case "application/json":
        request = this.text();
        from = "fromJSON";
        break;
      // @ts-expect-error fall through
      case "text/tab-separated-values":
        if (options?.delimiter === void 0)
          options = { ...options, delimiter: "	" };
      // fall through
      case "text/csv":
        request = this.text();
        from = "fromCSV";
        break;
      default:
        if (/\.arrow$/i.test(this.name)) {
          request = this.arrow();
          from = "fromArrow";
        } else if (/\.parquet$/i.test(this.name)) {
          request = this.parquet();
          from = "fromArrow";
        } else {
          throw new Error(`unable to determine Arquero loader: ${this.name}`);
        }
        break;
    }
    const [aq2, body] = await Promise.all([__vitePreload(() => import("https://cdn.jsdelivr.net/npm/arquero/+esm"), true ? [] : void 0), request]);
    return aq2[from](body, options);
  }
  async parquet() {
    const [Arrow2, Parquet, buffer] = await Promise.all([__vitePreload(() => import("https://cdn.jsdelivr.net/npm/apache-arrow@17.0.0/+esm"), true ? [] : void 0), __vitePreload(() => import("https://cdn.jsdelivr.net/npm/parquet-wasm/+esm"), true ? [] : void 0).then(async (Parquet2) => (await Parquet2.default("https://cdn.jsdelivr.net/npm/parquet-wasm/esm/parquet_wasm_bg.wasm"), Parquet2)), this.arrayBuffer()]);
    return Arrow2.tableFromIPC(Parquet.readParquet(new Uint8Array(buffer)).intoIPCStream());
  }
  async xml(mimeType = "application/xml") {
    return new DOMParser().parseFromString(await this.text(), mimeType);
  }
  async html() {
    return this.xml("text/html");
  }
}
function guessMimeType(name) {
  const i = name.lastIndexOf(".");
  const j = name.lastIndexOf("/");
  const extension = i > 0 && (j < 0 || i > j) ? name.slice(i).toLowerCase() : "";
  switch (extension) {
    case ".csv":
      return "text/csv";
    case ".tsv":
      return "text/tab-separated-values";
    case ".json":
      return "application/json";
    case ".html":
      return "text/html";
    case ".xml":
      return "application/xml";
    case ".png":
      return "image/png";
    case ".jpg":
      return "image/jpg";
    case ".js":
      return "text/javascript";
    default:
      return "application/octet-stream";
  }
}
class FileAttachmentImpl extends AbstractFile {
  constructor(href, name, mimeType, lastModified, size) {
    super(name, mimeType, lastModified, size);
    Object.defineProperty(this, "href", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "href", { value: href });
  }
}
Object.defineProperty(FileAttachmentImpl, "name", { value: "FileAttachment" });
FileAttachment.prototype = FileAttachmentImpl.prototype;
function fileAttachments(resolve2) {
  function FileAttachment2(name) {
    const result = resolve2(name += "");
    if (result == null)
      throw new Error(`File not found: ${name}`);
    if (typeof result === "object" && "url" in result) {
      const { url, mimeType } = result;
      return new FileAttachmentImpl(url, name, mimeType);
    }
    return new FileAttachmentImpl(result, name);
  }
  FileAttachment2.prototype = FileAttachmentImpl.prototype;
  return FileAttachment2;
}
function sluggify(string, { length = 50, fallback = "untitled", separator = "-" } = {}) {
  const parts = string.normalize("NFD").replace(/[\u0300-\u036f'‘’]/g, "").toLowerCase().split(/\W+/g).filter(nonempty);
  let i = -1;
  for (let l = 0, n = parts.length; ++i < n; ) {
    if ((l += parts[i].length) + i > length) {
      parts[i] = parts[i].substring(0, length - l + parts[i].length - i);
      break;
    }
  }
  return parts.slice(0, i + 1).filter(Boolean).join(separator) || fallback.slice(0, length);
}
function nonempty(string) {
  return string.length > 0;
}
async function sha256(input2) {
  const encoded = new TextEncoder().encode(input2);
  const buffer = await crypto.subtle.digest("SHA-256", encoded);
  return new Uint8Array(buffer).reduce((i, byte) => i << 8n | BigInt(byte), 0n);
}
function base36(int, length) {
  return int.toString(36).padStart(length, "0").slice(0, length);
}
async function hash(strings, ...params) {
  return base36(await sha256(JSON.stringify([strings, ...params])), 16);
}
async function stringHash(string) {
  return base36(await sha256(string), 16);
}
async function nameHash(name) {
  return /^[\w-]+$/.test(name) ? name : `${sluggify(basename(name))}.${base36(await sha256(name), 8)}`;
}
function basename(name) {
  return name.replace(/^.*\//, "");
}
const DatabaseClient = (name, options) => {
  return new DatabaseClientImpl(name, normalizeOptions$1(options));
};
function normalizeOptions$1({ id, since } = {}) {
  const options = {};
  if (id !== void 0)
    options.id = id;
  if (since !== void 0)
    options.since = new Date(since);
  return options;
}
class DatabaseClientImpl {
  constructor(name, options) {
    Object.defineProperty(this, "name", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "options", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperties(this, {
      name: { value: name, enumerable: true },
      options: { value: options, enumerable: true }
    });
  }
  async sql(strings, ...params) {
    const path = await this.cachePath(strings, ...params);
    const response = await fetch(path);
    if (!response.ok)
      throw new Error(`failed to fetch: ${path}`);
    return await response.json().then(revive);
  }
  async cachePath(strings, ...params) {
    return `.observable/cache/${await nameHash(this.name)}-${await hash(strings, ...params)}.json`;
  }
}
function revive({ rows, schema, date, ...meta }) {
  for (const column of schema) {
    switch (column.type) {
      case "bigint": {
        const { name } = column;
        for (const row of rows) {
          const value = row[name];
          if (value == null)
            continue;
          row[name] = Number(value);
        }
        break;
      }
      case "date": {
        const { name } = column;
        for (const row of rows) {
          const value = row[name];
          if (value == null)
            continue;
          row[name] = asDate(value);
        }
        break;
      }
    }
  }
  if (date != null)
    date = new Date(date);
  return Object.assign(rows, { schema, date }, meta);
}
function asDate(value) {
  return new Date(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}(?::\d{2})?$/.test(value) ? value + "Z" : value);
}
DatabaseClient.revive = revive;
DatabaseClient.prototype = DatabaseClientImpl.prototype;
Object.defineProperty(DatabaseClientImpl, "name", { value: "DatabaseClient" });
function canvas(width2, height) {
  const canvas2 = document.createElement("canvas");
  canvas2.width = width2;
  canvas2.height = height;
  return canvas2;
}
function context2d(width2, height, dpi = devicePixelRatio) {
  const canvas2 = document.createElement("canvas");
  canvas2.width = width2 * dpi;
  canvas2.height = height * dpi;
  canvas2.style.width = `${width2}px`;
  const context = canvas2.getContext("2d");
  context.scale(dpi, dpi);
  return context;
}
function svg$1(width2, height) {
  const svg2 = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg2.setAttribute("viewBox", `0 0 ${width2} ${height}`);
  svg2.setAttribute("width", `${width2}`);
  svg2.setAttribute("height", `${height}`);
  return svg2;
}
function text$1(value) {
  return document.createTextNode(value);
}
let count = 0;
function uid(name) {
  return new Id(`O-${name == null ? "" : `${name}-`}${++count}`);
}
class Id {
  constructor(id) {
    Object.defineProperty(this, "id", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "href", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.id = id;
    this.href = new URL(`#${id}`, location.href).href;
  }
  toString() {
    return `url(${this.href})`;
  }
}
const DOM = /* @__PURE__ */ Object.freeze(/* @__PURE__ */ Object.defineProperty({
  __proto__: null,
  canvas,
  context2d,
  svg: svg$1,
  text: text$1,
  uid
}, Symbol.toStringTag, { value: "Module" }));
function getInterpreterExtension(format2) {
  switch (format2) {
    case "html":
    case "text":
      return ".txt";
    case "jpeg":
      return ".jpg";
    case "json":
    case "arrow":
    case "parquet":
    case "csv":
    case "tsv":
    case "png":
    case "gif":
    case "svg":
    case "webp":
    case "xml":
      return `.${format2}`;
    default:
      return ".bin";
  }
}
const Interpreter = (name, options) => {
  return new InterpreterImpl(name, normalizeOptions(options));
};
function normalizeOptions({ format: format2 = "buffer", id, since } = {}) {
  const options = { format: format2 };
  if (id !== void 0)
    options.id = id;
  if (since !== void 0)
    options.since = new Date(since);
  return options;
}
class InterpreterImpl {
  constructor(name, options) {
    Object.defineProperty(this, "name", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "options", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperties(this, {
      name: { value: name, enumerable: true },
      options: { value: options, enumerable: true }
    });
  }
  async run(input2) {
    return FileAttachment(await this.cachePath(input2));
  }
  async cachePath(input2) {
    const { format: format2 } = this.options;
    return `.observable/cache/${await nameHash(this.name)}-${await stringHash(input2)}${getInterpreterExtension(format2)}`;
  }
}
Interpreter.prototype = InterpreterImpl.prototype;
Object.defineProperty(InterpreterImpl, "name", { value: "Interpreter" });
var __classPrivateFieldGet = function(receiver, state, kind, f) {
  if (kind === "a" && !f) throw new TypeError("Private accessor was defined without a getter");
  if (typeof state === "function" ? receiver !== state || !f : !state.has(receiver)) throw new TypeError("Cannot read private member from an object whose class did not declare it");
  return kind === "m" ? f : kind === "a" ? f.call(receiver) : f ? f.value : state.get(receiver);
};
var __classPrivateFieldSet = function(receiver, state, value, kind, f) {
  if (kind === "m") throw new TypeError("Private method is not writable");
  if (kind === "a" && !f) throw new TypeError("Private accessor was defined without a setter");
  if (typeof state === "function" ? receiver !== state || !f : !state.has(receiver)) throw new TypeError("Cannot write private member to an object whose class did not declare it");
  return kind === "a" ? f.call(receiver, value) : f ? f.value = value : state.set(receiver, value), value;
};
var _Observer_promise;
class Observer {
  constructor() {
    _Observer_promise.set(this, void 0);
    Object.defineProperty(this, "fulfilled", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "rejected", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    this.next();
  }
  async next() {
    const value = await __classPrivateFieldGet(this, _Observer_promise, "f");
    __classPrivateFieldSet(this, _Observer_promise, new Promise((res, rej) => (this.fulfilled = res, this.rejected = rej)), "f");
    return { done: false, value };
  }
  throw() {
    return { done: true };
  }
  return() {
    return { done: true };
  }
}
_Observer_promise = /* @__PURE__ */ new WeakMap();
const _ = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/lodash/+esm"), true ? [] : void 0).then((_2) => _2.default);
const aq = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/arquero/+esm"), true ? [] : void 0);
const Arrow = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/apache-arrow@17.0.0/+esm"), true ? [] : void 0);
const d3 = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/d3/+esm"), true ? [] : void 0);
const dot = () => __vitePreload(() => import("./dot-8mGemiIy.js"), true ? [] : void 0).then((_2) => _2.dot);
const duckdb = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@1.29.0/+esm"), true ? [] : void 0);
const DuckDBClient = () => __vitePreload(() => import("./duckdb-BUVS3D3i.js"), true ? [] : void 0).then((_2) => _2.DuckDBClient);
const echarts = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/echarts/dist/echarts.esm.min.js/+esm"), true ? [] : void 0);
const htl = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/htl/+esm"), true ? [] : void 0);
const html = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/htl/+esm"), true ? [] : void 0).then((_2) => _2.html);
const svg = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/htl/+esm"), true ? [] : void 0).then((_2) => _2.svg);
const Inputs = () => __vitePreload(() => import("./inputs-vIc-jKci.js"), true ? __vite__mapDeps([0,1]) : void 0);
const L = () => __vitePreload(() => import("./leaflet-CkvVhxBL.js"), true ? [] : void 0);
const mapboxgl = () => __vitePreload(() => import("./mapboxgl-C0i2HzjJ.js"), true ? [] : void 0).then((_2) => _2.default);
const md = () => __vitePreload(() => import("./md-DS0M2TH3.js"), true ? [] : void 0).then((_2) => _2.md);
const mermaid = () => __vitePreload(() => import("./mermaid-BmdbBLu8.js"), true ? [] : void 0).then((_2) => _2.mermaid);
const Plot = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/@observablehq/plot/+esm"), true ? [] : void 0);
const React = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/react/+esm"), true ? [] : void 0);
const ReactDOM = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/react-dom/client/+esm"), true ? [] : void 0);
const tex = () => __vitePreload(() => import("./tex-BsUqWRJ8.js"), true ? [] : void 0).then((_2) => _2.tex);
const topojson = () => __vitePreload(() => import("https://cdn.jsdelivr.net/npm/topojson-client/+esm"), true ? [] : void 0);
const vl = () => __vitePreload(() => import("./vega-lite-CESXoehe.js"), true ? [] : void 0).then((_2) => _2.vl);
const recommendedLibraries = /* @__PURE__ */ Object.freeze(/* @__PURE__ */ Object.defineProperty({
  __proto__: null,
  Arrow,
  DuckDBClient,
  Inputs,
  L,
  Plot,
  React,
  ReactDOM,
  _,
  aq,
  d3,
  dot,
  duckdb,
  echarts,
  htl,
  html,
  mapboxgl,
  md,
  mermaid,
  svg,
  tex,
  topojson,
  vl
}, Symbol.toStringTag, { value: "Module" }));
require$1.resolve = resolve;
function require$1(...specifiers) {
  return specifiers.length === 1 ? import(
    /* @vite-ignore */
    resolve(specifiers[0])
  ) : Promise.all(specifiers.map((s) => require$1(s))).then((modules) => Object.assign({}, ...modules));
}
function parseNpmSpecifier(specifier) {
  const parts = specifier.split("/");
  const namerange = specifier.startsWith("@") ? [parts.shift(), parts.shift()].join("/") : parts.shift();
  const ranged = namerange.indexOf("@", 1);
  const name = ranged > 0 ? namerange.slice(0, ranged) : namerange;
  const range = ranged > 0 ? namerange.slice(ranged) : "";
  const path = parts.length > 0 ? `/${parts.join("/")}` : "";
  return { name, range, path };
}
function resolve(_specifier) {
  const specifier = String(_specifier);
  if (isProtocol(specifier) || isLocal(specifier))
    return specifier;
  const { name, range, path } = parseNpmSpecifier(specifier);
  return `https://cdn.jsdelivr.net/npm/${name}${range}${path + (isFile(path) || isDirectory(path) ? "" : "/+esm")}`;
}
function isProtocol(specifier) {
  return /^\w+:/.test(specifier);
}
function isLocal(specifier) {
  return /^(\.\/|\.\.\/|\/)/.test(specifier);
}
function isFile(specifier) {
  return /(\.\w*)$/.test(specifier);
}
function isDirectory(specifier) {
  return /(\/)$/.test(specifier);
}
const aapl = () => csv(dataset("aapl.csv"));
const alphabet = () => csv(dataset("alphabet.csv"));
const cars = () => csv(dataset("cars.csv"));
const citywages = () => csv(dataset("citywages.csv"));
const diamonds = () => csv(dataset("diamonds.csv"));
const flare = () => csv(dataset("flare.csv"));
const industries = () => csv(dataset("industries.csv"));
const miserables = () => json(dataset("miserables.json"));
const olympians = () => csv(dataset("olympians.csv"));
const penguins = () => csv(dataset("penguins.csv"));
const pizza = () => csv(dataset("pizza.csv"));
const weather = () => csv(dataset("weather.csv"));
function dataset(name) {
  return `https://cdn.jsdelivr.net/npm/@observablehq/sample-datasets/${name}`;
}
async function json(url) {
  const response = await fetch(url);
  if (!response.ok)
    throw new Error(`unable to fetch ${url}: status ${response.status}`);
  return response.json();
}
async function text(url) {
  const response = await fetch(url);
  if (!response.ok)
    throw new Error(`unable to fetch ${url}: status ${response.status}`);
  return response.text();
}
async function csv(url, typed) {
  const [contents, d32] = await Promise.all([text(url), __vitePreload(() => import("https://cdn.jsdelivr.net/npm/d3-dsv/+esm"), true ? [] : void 0)]);
  return d32.csvParse(contents, d32.autoType);
}
const sampleDatasets = /* @__PURE__ */ Object.freeze(/* @__PURE__ */ Object.defineProperty({
  __proto__: null,
  aapl,
  alphabet,
  cars,
  citywages,
  diamonds,
  flare,
  industries,
  miserables,
  olympians,
  penguins,
  pizza,
  weather
}, Symbol.toStringTag, { value: "Module" }));
const root = document.querySelector("main") ?? document.body;
const library = {
  now: () => now(),
  width: () => width(root),
  DatabaseClient: () => DatabaseClient,
  FileAttachment: () => FileAttachment,
  Generators: () => Generators,
  Interpreter: () => Interpreter,
  Mutable: () => Mutable,
  DOM: () => DOM,
  // deprecated!
  require: () => require$1,
  // deprecated!
  __ojs_observer: () => () => new Observer(),
  ...recommendedLibraries,
  ...sampleDatasets
};
class NotebookRuntime {
  constructor(builtins = library) {
    Object.defineProperty(this, "runtime", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    Object.defineProperty(this, "main", {
      enumerable: true,
      configurable: true,
      writable: true,
      value: void 0
    });
    const runtime = new Runtime({ ...builtins, __ojs_runtime: () => runtime });
    this.runtime = Object.assign(runtime, { fileAttachments });
    this.main = runtime.module();
  }
  define(state, definition, observer) {
    define$1(this.main, state, definition, observer);
  }
}
const defaultNotebook = new NotebookRuntime();
defaultNotebook.runtime;
const main = defaultNotebook.main;
const define = defaultNotebook.define.bind(defaultNotebook);
main.constructor.prototype.defines = function(name) {
  return this._scope.has(name) || this._builtins.has(name) || this._runtime._builtin._scope.has(name);
};
define(
  {
    root: document.getElementById(`cell-2`),
    expanded: [],
    variables: []
  },
  {
    id: 2,
    body: (FileAttachment2) => {
      return FileAttachment2(new URL("/opa/benchmarks/assets/node-2sqbms3s85uuh6dw-DeIElMAg.json", import.meta.url).href).json();
    },
    inputs: ["FileAttachment"],
    outputs: [],
    output: "benchmarks",
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-3`),
    expanded: [],
    variables: []
  },
  {
    id: 3,
    body: (FileAttachment2) => {
      return FileAttachment2(new URL("/opa/benchmarks/assets/node-10x93cm4f8cnpem6-DkFBUfNy.json", import.meta.url).href).json();
    },
    inputs: ["FileAttachment"],
    outputs: [],
    output: "commits",
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-4`),
    expanded: [],
    variables: []
  },
  {
    id: 4,
    body: (FileAttachment2) => {
      return FileAttachment2(new URL("/opa/benchmarks/assets/node-2wq9arc6vpxqm3ub-B0Dhiz4s.json", import.meta.url).href).json();
    },
    inputs: ["FileAttachment"],
    outputs: [],
    output: "tags",
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-5`),
    expanded: [],
    variables: []
  },
  {
    id: 5,
    body: (benchmarks, tags) => {
      function flatten(benchmarksData, tagData) {
        return benchmarksData.flatMap((commitData) => {
          const commit = commitData.Version;
          const date = new Date(commitData.Date * 1e3);
          return (commitData.Suites || []).flatMap((suite) => {
            const pkg = suite.Pkg.replace("github.com/open-policy-agent/opa", ".");
            return (suite.Benchmarks || []).flatMap((benchmark) => {
              const base = { commit, date, pkg, name: benchmark.Name, tag: tagData[commit] };
              const r0 = {
                ...base,
                measure: "NsPerOp",
                value: benchmark.NsPerOp
              };
              const r1 = {
                ...base,
                measure: "AllocsPerOp",
                value: benchmark.Mem.AllocsPerOp
              };
              const r2 = {
                ...base,
                measure: "BytesPerOp",
                value: benchmark.Mem.BytesPerOp
              };
              return [r0, r1, r2];
            });
          });
        });
      }
      const rows = flatten(benchmarks, tags);
      return { flatten, rows };
    },
    inputs: ["benchmarks", "tags"],
    outputs: ["flatten", "rows"],
    output: void 0,
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-6`),
    expanded: [],
    variables: []
  },
  {
    id: 6,
    body: (rows, view, Inputs2) => {
      const tagsInData = [...new Set(rows.flatMap((d) => d.tag || []))];
      const selectedTag = view(Inputs2.select(tagsInData, { label: "Tag as Index:" }));
      return { tagsInData, selectedTag };
    },
    inputs: ["rows", "view", "Inputs"],
    outputs: ["tagsInData", "selectedTag"],
    output: void 0,
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-7`),
    expanded: [],
    variables: []
  },
  {
    id: 7,
    body: (d32, rows, htl2, view, Inputs2) => {
      const relativeDifferencesByMeasure = d32.rollup(
        rows,
        (v) => {
          v.sort((a, b) => b.date.getTime() - a.date.getTime());
          const lastMeasurement = v[0];
          const latestTaggedMeasurement = v.find((d) => !!d.tag);
          if (!latestTaggedMeasurement) return null;
          return { relativeDifference: latestTaggedMeasurement.value !== 0 ? lastMeasurement.value / latestTaggedMeasurement.value : null };
        },
        (d) => d.pkg,
        (d) => d.name,
        (d) => d.measure
      );
      const benchmarksWithRelDiffs = [];
      for (const [pkg, nameMap] of relativeDifferencesByMeasure) {
        for (const [name, measureMap] of nameMap) {
          const outputRow = {
            pkg,
            name
          };
          for (const [measure, data] of measureMap) {
            if (data && data?.relativeDifference !== null) {
              outputRow[measure] = data.relativeDifference;
            }
          }
          if (Object.keys(outputRow).length > 2) {
            benchmarksWithRelDiffs.push(outputRow);
          }
        }
      }
      d32.sort(benchmarksWithRelDiffs, (d) => d.NsPerOp);
      const numericKeys = ["NsPerOp", "BytesPerOp", "AllocsPerOp"];
      const minMaxByMeasureKeyLog2 = /* @__PURE__ */ new Map();
      numericKeys.forEach((key) => {
        let colMinLog2 = Infinity;
        let colMaxLog2 = -Infinity;
        benchmarksWithRelDiffs.forEach((row) => {
          const value = row[key];
          if (typeof value === "number" && !isNaN(value) && value > 0) {
            const log2Value = Math.log2(value);
            colMinLog2 = Math.min(colMinLog2, log2Value);
            colMaxLog2 = Math.max(colMaxLog2, log2Value);
          }
        });
        minMaxByMeasureKeyLog2.set(key, { minLog2: colMinLog2, maxLog2: colMaxLog2 });
      });
      const scalesByMeasureKey = /* @__PURE__ */ new Map();
      numericKeys.forEach((key) => {
        const { minLog2: colMinLog2, maxLog2: colMaxLog2 } = minMaxByMeasureKeyLog2.get(key);
        const maxAbsDeviationLog2 = Math.max(Math.abs(colMinLog2 - 0), Math.abs(colMaxLog2 - 0));
        const symmetricMinLog2 = 0 - maxAbsDeviationLog2;
        const symmetricMaxLog2 = 0 + maxAbsDeviationLog2;
        const reversedRdYlGn = (t) => d32.interpolateRdYlGn(1 - t);
        const colScale = d32.scaleDiverging(reversedRdYlGn).domain([symmetricMinLog2, 0, symmetricMaxLog2]);
        scalesByMeasureKey.set(key, colScale);
      });
      function colorScaleBackground(key, precision = 2, addPaddingRight = true) {
        const scale = scalesByMeasureKey.get(key);
        return (value) => {
          if (typeof value !== "number" || isNaN(value) || value === null) {
            return value;
          }
          const bgColor = scale(Math.log2(value));
          const textColor = d32.color(bgColor).rgb().r * 0.299 + d32.color(bgColor).rgb().g * 0.587 + d32.color(bgColor).rgb().b * 0.114 > 186 ? "black" : "white";
          return htl2.html`<div style="
    background-color: ${bgColor};
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: end;
    box-sizing: border-box;
    ${addPaddingRight ? "padding-right: 3px;" : ""} /* Optional padding */
    color: ${textColor};
  ">${value.toFixed(precision)}</div>`;
        };
      }
      function termFilter(term) {
        return new RegExp(escapeRegExp(term), "iu");
      }
      function escapeRegExp(text2) {
        return text2.replace(/[\\^$.*+?()[\]{}]/g, "\\$&");
      }
      function columnFilter(columns) {
        return (query) => {
          const filters = `${query}`.split(/\s+/g).filter((t) => t).map(termFilter);
          return (d) => {
            out: for (const filter of filters) {
              for (const column of columns) {
                if (filter.test(d[column])) {
                  continue out;
                }
              }
              return false;
            }
            return true;
          };
        };
      }
      const search = view(Inputs2.search(benchmarksWithRelDiffs, {
        label: "Benchmarks",
        filter: columnFilter(["pkg", "name"]),
        autocomplete: true
      }));
      return { relativeDifferencesByMeasure, benchmarksWithRelDiffs, numericKeys, minMaxByMeasureKeyLog2, scalesByMeasureKey, colorScaleBackground, termFilter, escapeRegExp, columnFilter, search };
    },
    inputs: ["d3", "rows", "htl", "view", "Inputs"],
    outputs: ["relativeDifferencesByMeasure", "benchmarksWithRelDiffs", "numericKeys", "minMaxByMeasureKeyLog2", "scalesByMeasureKey", "colorScaleBackground", "termFilter", "escapeRegExp", "columnFilter", "search"],
    output: void 0,
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-8`),
    expanded: [],
    variables: []
  },
  {
    id: 8,
    body: (view, Inputs2, search, benchmarksWithRelDiffs, colorScaleBackground) => {
      const selectedRow = view(Inputs2.table(search.length > 0 ? search : benchmarksWithRelDiffs, {
        multiple: false,
        value: search.length > 0 ? search[0] : benchmarksWithRelDiffs[0],
        width: {
          pkg: 200,
          name: 300
        },
        sort: "NsPerOp",
        reverse: true,
        format: {
          NsPerOp: colorScaleBackground("NsPerOp"),
          AllocsPerOp: colorScaleBackground("AllocsPerOp"),
          BytesPerOp: colorScaleBackground("BytesPerOp")
        }
      }));
      return { selectedRow };
    },
    inputs: ["view", "Inputs", "search", "benchmarksWithRelDiffs", "colorScaleBackground"],
    outputs: ["selectedRow"],
    output: void 0,
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-9`),
    expanded: [],
    variables: []
  },
  {
    id: 9,
    body: (selectedRow, d32, rows, selectedTag, Plot2, tags, tagsInData, display2) => {
      let plot;
      if (selectedRow) {
        const data = d32.sort(rows.filter((d) => d.name === selectedRow.name && d.pkg == selectedRow.pkg), (d) => d.date);
        const idx = data.findIndex((d) => d.tag == selectedTag) || 0;
        const xy = Plot2.normalizeY({
          // NB(sr) We know that measurements follow each other in data, so it's safe
          // to use the first `values` element as offset:
          basis: (values, data2) => data2[idx + values[0]],
          x: (d) => d.tag || d.commit,
          y: "value",
          z: "measure"
        });
        plot = Plot2.plot({
          width: 1024,
          margin: 80,
          y: {
            type: "log",
            base: 2,
            label: "Performance Change (log₂)",
            tickFormat: (d) => {
              if (d === 1) return "Base";
              if (d > 1) return `+${((d - 1) * 100).toFixed(0)}%`;
              if (d < 1) return `-${((1 - d) * 100).toFixed(0)}%`;
            },
            grid: true,
            ticks: 8
          },
          x: {
            type: "band",
            domain: data.map((d) => tags[d.commit] || d.commit),
            tickFormat: (d) => d.substring(0, 7),
            tickRotate: -45,
            tickAnchor: "start",
            clip: false
          },
          height: 340,
          color: { legend: true },
          marks: [
            Plot2.dotY(data, {
              ...xy,
              fill: "measure",
              href: (d) => `https://github.com/open-policy-agent/opa/commit/${d.commit}`,
              target: "_blank",
              channels: {
                absolute: "value"
              },
              tip: {
                format: {
                  y: (d) => {
                    if (d === 1) return "Base";
                    if (d > 1) return `+${((d - 1) * 100).toFixed(0)}%`;
                    if (d < 1) return `-${((1 - d) * 100).toFixed(0)}%`;
                    return "";
                  },
                  x: false,
                  fill: true,
                  date: true,
                  absolute: true
                }
              }
            }),
            Plot2.lineY(data, {
              ...xy,
              stroke: "measure",
              curve: "step",
              tip: false
            }),
            Plot2.ruleX(tagsInData, {
              x: (d) => d,
              stroke: "grey",
              strokeDasharray: "4,4",
              strokeOpacity: 0.7
            }),
            Plot2.ruleX(data, Plot2.pointerX({
              x: xy.x,
              py: xy.y,
              strokeDasharray: "4,4",
              strokeOpacity: 0.7,
              stroke: "grey"
            }))
          ]
        });
        display2(plot);
      }
      return { plot };
    },
    inputs: ["selectedRow", "d3", "rows", "selectedTag", "Plot", "tags", "tagsInData", "display"],
    outputs: ["plot"],
    output: void 0,
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-10`),
    expanded: [],
    variables: []
  },
  {
    id: 10,
    body: (Generators2, plot) => {
      const x = Generators2.input(plot);
      return { x };
    },
    inputs: ["Generators", "plot"],
    outputs: ["x"],
    output: void 0,
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-11`),
    expanded: [],
    variables: []
  },
  {
    id: 11,
    body: (commits, x) => {
      const commit = commits[x?.commit];
      return { commit };
    },
    inputs: ["commits", "x"],
    outputs: ["commit"],
    output: void 0,
    assets: void 0,
    autodisplay: false,
    autoview: void 0,
    automutable: void 0
  }
);
define(
  {
    root: document.getElementById(`cell-12`),
    expanded: [],
    variables: []
  },
  {
    id: 12,
    body: (md2, commit) => {
      return md2`\`\`\`
${commit ? `Author: ${commit?.github_username}
Date: ${commit?.author_date}

${commit?.message}` : ""}
\`\`\``;
    },
    inputs: ["md", "commit"],
    outputs: [],
    output: void 0,
    assets: void 0,
    autodisplay: true,
    autoview: false,
    automutable: void 0
  }
);
export {
  __vitePreload as _
};
