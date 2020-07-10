# OPA Fuzzer

## How To Run
Install the fuzzer:

```bash
make deps
```

Build the fuzzer package. The fuzzer package includes the code to run and the corpus:

```bash
make build
```

Run the fuzzer:

```bash
make fuzz
```

The last command will start the fuzzer and output the results to the workdir.

> Note: The fuzzer will run in an infinite loop testing until manually interrupted!

See [go-fuzz/README.md](https://github.com/dvyukov/go-fuzz) for details on the
fuzzer output. Pay attention to the `restarts` output field. This value should
be around 1/10,000. If it's higher than this and the `crashers` field is greater
than zero, check the output directory for crash output.

## Easy Mode
```bash
make all
```

Starts the unbounded fuzzing process.
