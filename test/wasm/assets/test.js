const assert = require('assert');

const { readFileSync, readdirSync } = require('fs');

function stringDecoder(mem) {
    return function (addr) {
        const i8 = new Int8Array(mem.buffer);
        var s = "";
        while (i8[addr] != 0) {
            s += String.fromCharCode(i8[addr++]);
        }
        return s;
    }
}

function red(text) {
    return '\x1b[0m\x1b[31m' + text + '\x1b[0m';
}

function green(text) {
    return '\x1b[0m\x1b[32m' + text + '\x1b[0m';
}

function yellow(text) {
    return '\x1b[0m\x1b[33m' + text + '\x1b[0m';
}

const PASS = 'PASS';
const FAIL = 'FAIL';
const ERROR = 'ERROR';

function report(state, name, msg, extra) {
    if (state === PASS) {
        if (process.env.VERBOSE === '1') {
            console.log(green('PASS'), name);
            return true;
        }
        return false;
    } else if (state === FAIL) {
        console.log(yellow('FAIL'), name + ':', msg);
    } else {
        console.log(red('ERROR'), name + ':', msg);
    }
    if (extra !== '') {
        console.log(extra);
    }
    return true
}

function skip(name, msg) {
    if (process.env.VERBOSE === '1') {
        console.log(yellow('SKIP'), name + ':', msg);
    }
}

function now() {
    const [sec, nsec] = process.hrtime();
    return (sec * 1000 * 1000) + (nsec / 1000);
}

function formatMicros(us) {
    if (us <= 1000) {
        return us + 'µs'
    } else if (us <= 1000 * 1000) {
        return (us / 1000).toFixed(4) + 'ms'
    } else {
        return (us / (1000 * 1000)).toFixed(4) + 's'
    }
}

function loadJSON(mod, value) {

    if (value === undefined) {
        return 0;
    }

    const str = JSON.stringify(value);
    const rawAddr = mod.instance.exports.opa_malloc(str.length);
    const buf = new Uint8Array(mod.instance.exports.memory.buffer);

    for (let i = 0; i < str.length; i++) {
        buf[rawAddr + i] = str.charCodeAt(i);
    }

    const parsedAddr = mod.instance.exports.opa_json_parse(rawAddr, str.length);

    if (parsedAddr == 0) {
        throw "failed to parse json value";
    }

    return parsedAddr;
}

function dumpJSON(mod, addr) {
    const rawAddr = mod.instance.exports.opa_json_dump(addr);
    return parseJSON(mod.instance.exports.memory, rawAddr);
}

function parseJSON(memory, rawAddr) {
    const buf = new Uint8Array(memory.buffer);
    const idx = rawAddr + buf.slice(rawAddr).findIndex((elem) => elem === 0);
    // TODO(sr): use TextDecoder and friends
    return JSON.parse(decodeURIComponent(escape(String.fromCharCode.apply(null, buf.slice(rawAddr, idx)))));
}

function builtinCustomTest(a) {
    return a + 1;
}

function builtinCustomTestImpure() {
    return "foo";
}

var run = false;

function builtinCustomTestMemoization() {
    if (run) {
      throw "should have been memoized";
    }
    run = true
    return 100;
}

const builtinFuncs = {
    custom_builtin_test: builtinCustomTest,
    custom_builtin_test_impure: builtinCustomTestImpure,
    custom_builtin_test_memoization: builtinCustomTestMemoization,
}

// builtinCall dispatches the built-in function. Arguments are deserialized from
// JSON into JavaScript values and the result is serialized for passing back
// into Wasm.
function builtinCall(policy, func) {

    const impl = builtinFuncs[policy.builtins[func]];

    if (impl === undefined) {
        throw { message: "not implemented: built-in " + func + ": " + policy.builtins[func] }
    }

    var argArray = Array.prototype.slice.apply(arguments);
    let args = [];

    for (let i = 2; i < argArray.length; i++) {
        const jsArg = dumpJSON(policy.module, argArray[i]);
        args.push(jsArg);
    }

    const result = impl(...args);

    return loadJSON(policy.module, result);
}

async function instantiate(bytes, data) {

    const memory = new WebAssembly.Memory({initial: 5});
    let addr2string = () => console.warn("cannot call addr2string from Start function");
    const policy = {};

    policy.module = await WebAssembly.instantiate(bytes, {
        env: {
            memory,
            opa_abort: (addr) => {
                throw { message: addr2string(addr) };
            },
            opa_println: (addr) => {
                console.log(addr2string(addr));
            },
            opa_builtin0: (func, _ctx)                 => builtinCall(policy, func),
            opa_builtin1: (func, _ctx, v1)             => builtinCall(policy, func, v1),
            opa_builtin2: (func, _ctx, v1, v2)         => builtinCall(policy, func, v1, v2),
            opa_builtin3: (func, _ctx, v1, v2, v3)     => builtinCall(policy, func, v1, v2, v3),
            opa_builtin4: (func, _ctx, v1, v2, v3, v4) => builtinCall(policy, func, v1, v2, v3, v4),
        },
    });

    addr2string = stringDecoder(policy.module.instance.exports.memory);

    builtins = dumpJSON(policy.module, policy.module.instance.exports.builtins());
    policy.builtins = {};

    for (var key of Object.keys(builtins)) {
        policy.builtins[builtins[key]] = key
    }

    policy.dataAddr = loadJSON(policy.module, data);
    policy.heapPtr = policy.module.instance.exports.opa_heap_ptr_get();

    return policy;
}

function evaluate(policy, input) {

    let inputLen = 0;
    let inputAddr = 0;
    if (input) {
        const inp = JSON.stringify(input);
        const buf = new Uint8Array(policy.module.instance.exports.memory.buffer);
        inputAddr = policy.heapPtr;
        inputLen = inp.length;

        for (let i = 0; i < inputLen; i++) {
            buf[inputAddr + i] = inp.charCodeAt(i);
        }
        policy.heapPtr = inputAddr + inputLen;
    }

    const addr = policy.module.instance.exports.opa_eval(
        0, // reserved
        0, // entrypoint
        policy.dataAddr,
        inputAddr,
        inputLen,
        policy.heapPtr,
        0, // json output
        );

    return { addr };
}

function namespace(cache, key) {
    if (key in cache) {
        cache[key] += 1;
        return key + ' (' + cache[key] + ')'
    } else {
        cache[key] = 0;
        return key;
    }
}

async function test() {

    const t0 = now();
    var testCases = [];
    const files = readdirSync('.');
    let numFiles = 0;

    files.forEach(file => {
        if (file.endsWith('.json')) {
            numFiles++;
            const testFile = JSON.parse(readFileSync(file));
            if (Array.isArray(testFile.cases)) {
                testFile.cases.forEach(testCase => {
                    testCase.note = file + ': ' + testCase.note;
                    if (testCase.wasm !== undefined) {
                        testCase.wasmBytes = Buffer.from(testCase.wasm, 'base64');
                    }
                    testCases.push(testCase);
                });
            }
        }
    })

    const t_load = now();
    const dt_load = t_load - t0;
    console.log(`Found ${testCases.length} WASM test cases in ${numFiles} file(s). Took ${formatMicros(dt_load)}. Running now.\n`);

    let numSkipped = 0;
    let numPassed = 0;
    let numFailed = 0;
    let numErrors = 0;
    let dirty = false;
    let cache = {};

    for (let i = 0; i < testCases.length; i++) {

        let state = 'FAIL';
        let name = namespace(cache, testCases[i].note);

        if (testCases[i].skip === true) {
            skip(name, testCases[i].skip_reason);
            numSkipped++;
            continue
        }

        let msg = '';
        let extra = '';

        try {
            const policy = await instantiate(testCases[i].wasmBytes, testCases[i].data);
            const result = evaluate(policy, testCases[i].input);

            const expDefined = testCases[i].want_defined;
            const rs = parseJSON(policy.module.instance.exports.memory, result.addr);

            if (expDefined !== undefined) {
                const len = rs.length
                if (expDefined) {
                    if (len > 0) {
                        state = PASS;
                    } else {
                        msg = 'expected non-empty/defined result';
                    }
                } else {
                    if (len == 0) {
                        state = PASS;
                    } else {
                        msg = 'expected empty/undefined result';
                    }
                }
            }

            const expResultSet = testCases[i].want_result;

            if (expResultSet !== undefined) {

                // Note: Resultset ordering does not matter.
                if (rs.length === expResultSet.length) {
                    let found = 0
                    expResultSet.forEach(expResult => {
                        for (let i = 0; i < rs.length; i++) {
                            try {
                                assert.deepStrictEqual(expResult, rs[i], "didn't match")
                                found++
                                break
                            } catch (e) {
                                // Ignore the error
                            }
                        }
                    })
                    if (expResultSet.length === found) {
                        state = PASS;
                    }
                }

                if (state !== PASS) {
                    msg = 'unexpected result';
                    extra = '\twant: ' + JSON.stringify(expResultSet) + '\n\tgot : ' + JSON.stringify(rs);
                }
            }

        } catch (e) {
            const exp = testCases[i].want_error;
            if (exp !== undefined && exp.length !== 0) {
                if (e.message.includes(exp)) {
                    state = PASS;
                } else {
                    state = ERROR;
                    msg = 'want: ' + yellow(exp) + ' but got: ' + red(e.message);
                }
            } else {
                state = ERROR;
                msg = e;
            }
        }

        if (state == PASS) {
            numPassed++;
        } else if (state === FAIL) {
            numFailed++;
        } else {
            numErrors++;
        }

        dirty = report(state, name, msg, extra) || dirty;
    }

    const t_end = now();
    const dt_end = t_end - t_load;

    if (dirty) {
        console.log();
    }

    console.log('SUMMARY:');
    console.log('--------');
    console.log('PASS:', numPassed + '/' + testCases.length);

    if (numFailed > 0) {
        console.log('FAIL:', numFailed + '/' + testCases.length);
    }

    if (numSkipped > 0) {
        console.log('SKIP:', numSkipped + '/' + testCases.length);
    }

    if (numErrors > 0) {
        console.log('ERROR:', numErrors + '/' + testCases.length);
    }

    console.log();
    console.log('TOOK:', formatMicros(dt_end));

    if ((numFailed + numErrors) > 0) {
        process.exit(1);
    }
}

test();
