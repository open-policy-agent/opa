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

function report(passed, error, msg) {
    if (passed === true) {
        if (process.env.VERBOSE === '1') {
            console.log(green('PASS'), msg);
            return true;
        }
        return false;
    } else if (error === undefined) {
        console.log(yellow('FAIL'), msg);
    } else {
        console.log(red('ERROR'), msg, error);
    }
    return true
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
                throw addr2string(addr);
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
    const returnCode = policy.module.instance.exports.eval(inputAddr, policy.dataAddr);
    return { returnCode: returnCode };
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
                    testCase.wasmBytes = Buffer.from(testCase.wasm, 'base64');
                    testCases.push(testCase);
                });
            }
        }
    })

    const t_load = now();
    const dt_load = t_load - t0;
    console.log('Found ' + testCases.length + ' WASM test cases in ' + numFiles + ' file(s). Took ' + formatMicros(dt_load) + '. Running now.');
    console.log();

    let numPassed = 0;
    let numFailed = 0;
    let numErrors = 0;
    let dirty = false;
    let cache = {};

    for (let i = 0; i < testCases.length; i++) {

        let passed = false;
        let error = undefined;

        try {
            const policy = await instantiate(testCases[i].wasmBytes, memory, testCases[i].data);
            const result = evaluate(policy, testCases[i].input);
            passed = result.returnCode === testCases[i].return_code;
        } catch (e) {
            if (testCases[i].want_error === undefined) {
                passed = false;
                error = e;
            } else if (e.message.includes(testCases[i].want_error)) {
                passed = true;
            } else {
                passed = false;
                error = e;
            }
        }

        if (passed) {
            numPassed++;
        } else if (error === undefined) {
            numFailed++;
        } else {
            numErrors++;
        }

        dirty = report(passed, error, namespace(cache, testCases[i].note)) || dirty;
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
