const { readFileSync } = require('fs');

function stringDecoder(mem) {
    return function(addr) {
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

    const memory = new WebAssembly.Memory({ initial: 3 });
    let addr2string;
    let cache = {};
    let failedOrErrored = 0;
    let seenFuncs = {};

    const module = await WebAssembly.instantiate(readFileSync(executable), {
        env: {
            memory,
            opa_builtin0: () => 0,
            opa_builtin1: () => 0,
            opa_builtin2: () => 0,
            opa_builtin3: () => 0,
            opa_builtin4: () => 0,
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

    addr2string = stringDecoder(module.instance.exports.memory);

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

    // NOTE(sr): seenFuncs will not contain all tests run, but only those that
    // actually call opa_test_{pass,fail}. However, if it's empty, something is
    // definitely wrong.
    if (Object.keys(seenFuncs).length == 0) {
        console.log(red('ERROR'), "no tests executed");
        process.exit(2);
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
