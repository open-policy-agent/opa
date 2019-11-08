const assert = require('assert');

const { readFileSync, readdirSync } = require('fs');

function stringDecoder(mem) {
    return function (addr) {
        const i8 = new Int8Array(mem.buffer);
        const start = addr;
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

function loadJSON(mod, memory, value) {

    if (value === undefined) {
        return 0;
    }

    const str = JSON.stringify(value);
    const rawAddr = mod.instance.exports.opa_malloc(str.length);
    const buf = new Uint8Array(memory.buffer);

    for (let i = 0; i < str.length; i++) {
        buf[rawAddr + i] = str.charCodeAt(i);
    }

    const parsedAddr = mod.instance.exports.opa_json_parse(rawAddr, str.length);

    if (parsedAddr == 0) {
        throw "failed to parse json value"
    }

    return parsedAddr;
}

async function instantiate(bytes, memory, data) {

    const addr2string = stringDecoder(memory);

    const mod = await WebAssembly.instantiate(bytes, {
        env: {
            memory: memory,
            opa_abort: function (addr) {
                throw { message: addr2string(addr) };
            },
            opa_println: function (addr) {
                console.log(addr2string(addr));
            },
        },
    });

    const dataAddr = loadJSON(mod, memory, data || {});
    const heapPtr = mod.instance.exports.opa_heap_ptr_get();
    const heapTop = mod.instance.exports.opa_heap_top_get();

    return { module: mod, memory: memory, heapPtr: heapPtr, heapTop: heapTop, dataAddr: dataAddr };
}

function evaluate(policy, input) {
    policy.module.instance.exports.opa_heap_ptr_set(policy.heapPtr);
    policy.module.instance.exports.opa_heap_top_set(policy.heapTop);
    const inputAddr = loadJSON(policy.module, policy.memory, input);
    const resultAddr = policy.module.instance.exports.eval(inputAddr, policy.dataAddr);
    return { addr: resultAddr };
}

function decodeResultSet(policy, result) {

    const rawAddr = policy.module.instance.exports.opa_json_dump(result.addr);
    const buf = new Uint8Array(policy.memory.buffer);

    // NOTE(tsandall): There must be a better way of doing this...
    let s = '';
    let idx = rawAddr;

    while (buf[idx] != 0) {
        s += String.fromCharCode(buf[idx++]);
    }

    return JSON.parse(s);
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

    const memory = new WebAssembly.Memory({ initial: 5 });
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
    console.log('Found ' + testCases.length + ' WASM test cases in ' + numFiles + ' file(s). Took ' + formatMicros(dt_load) + '. Running now.');
    console.log();

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
            const policy = await instantiate(testCases[i].wasmBytes, memory, testCases[i].data);
            const result = evaluate(policy, testCases[i].input);

            const expDefined = testCases[i].want_defined;

            if (expDefined !== undefined) {
                const len = policy.module.instance.exports.opa_value_length(result.addr);
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

                const rs = decodeResultSet(policy, result);

                try {
                    assert.deepStrictEqual(rs, expResultSet, 'unexpected result set');
                    state = PASS;
                } catch (e) {
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
