const { readFileSync } = require('fs');

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

function namespace(cache, func, note) {
    let key = func + note;
    if (key in cache) {
        cache[key] += 1;
        note = note + ' (' + cache[key] + ')'
    } else {
        cache[key] = 0;
    }
    return note;
}

function report(passed, error, msg) {
    if (passed === true) {
        if (process.env.VERBOSE === '1') {
            console.log(green('PASS'), msg);
        }
    } else if (error === undefined) {
        console.log(yellow('FAIL'), msg);
    } else {
        console.log(red('ERROR'), msg, error);
    }
}

async function test(executable) {

    const mem = new WebAssembly.Memory({ initial: 3 });
    const addr2string = stringDecoder(mem);

    let cache = {};
    let failedOrErrored = 0;
    let seenFuncs = {};

    const module = await WebAssembly.instantiate(readFileSync(executable), {
        env: {
            memory: mem,
            opa_builtin0: (_1, _2) => { return 0; },
            opa_builtin1: (_1, _2, _3, _4) => { return 0; },
            opa_builtin2: (_1, _2, _3, _4) => { return 0; },
            opa_builtin3: (_1, _2, _3, _4, _5) => { return 0; },
            opa_builtin4: (_1, _2, _3, _4, _5, _6) => { return 0; },
            opa_println: (msg) => {
                console.log(addr2string(msg));
            },
            opa_abort: (msg) => {
                throw 'abort: ' + addr2string(msg);
            },
            opa_test_pass: (note, func) => {
                note = addr2string(note);
                func = addr2string(func);
                note = namespace(cache, func, note);
                seenFuncs[func] = true;
                let key = func + '/' + note
                report(true, undefined, key);
            },
            opa_test_fail: (note, func, file, line) => {
                note = addr2string(note);
                func = addr2string(func);
                note = namespace(cache, func, note);
                seenFuncs[func] = true;
                let key = func + '/' + note;
                failedOrErrored++;
                report(false, undefined, key + ' ' + addr2string(file) + ':' + line);
            },
        }
    });

    for (let key in module.instance.exports) {
        if (key.startsWith("test_")) {
            try {
                module.instance.exports[key]();
                if (!(key in seenFuncs)) {
                    report(true, undefined, key);
                }
            } catch (e) {
                report(false, e, key)
            }
        }
    }

    if (failedOrErrored > 0) {
        process.exit(1);
    }
}

if (process.argv.length != 3) {
    console.log(process.argv[1] + " <test executable path>");
    process.exit(1);
}

test(process.argv[2]);
