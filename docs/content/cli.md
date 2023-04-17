---
title: CLI
kind: documentation
weight: 90
restrictedtoc: true
---

The OPA executable provides the following commands.

## opa bench

Benchmark a Rego query

### Synopsis

Benchmark a Rego query and print the results.

The benchmark command works very similar to 'eval' and will evaluate the query in the same fashion. The
evaluation will be repeated a number of times and performance results will be returned.

Example with bundle and input data:

	opa bench -b ./policy-bundle -i input.json 'data.authz.allow'

To enable more detailed analysis use the --metrics and --benchmem flags.

To run benchmarks against a running OPA server to evaluate server overhead use the --e2e flag.

The optional "gobench" output format conforms to the Go Benchmark Data Format.


```
opa bench <query> [flags]
```

### Options

```
      --benchmem                       report memory allocations with benchmark results (default true)
  -b, --bundle string                  set bundle file(s) or directory path(s). This flag can be repeated.
  -c, --config-file string             set path of configuration file
      --count int                      number of times to repeat each benchmark (default 1)
  -d, --data string                    set policy or data file(s). This flag can be repeated.
      --e2e                            run benchmarks against a running OPA server
      --fail                           exits with non-zero exit code on undefined/empty result and errors (default true)
  -f, --format {json,pretty,gobench}   set output format (default pretty)
  -h, --help                           help for bench
      --ignore strings                 set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)
      --import string                  set query import(s). This flag can be repeated.
  -i, --input string                   set input file path
      --metrics                        report query performance metrics (default true)
      --package string                 set query package
  -p, --partial                        perform partial evaluation
  -s, --schema string                  set schema file path or directory path
      --shutdown-grace-period int      set the time (in seconds) that the server will wait to gracefully shut down. This flag is valid in 'e2e' mode only. (default 10)
      --shutdown-wait-period int       set the time (in seconds) that the server will wait before initiating shutdown. This flag is valid in 'e2e' mode only.
      --stdin                          read query from stdin
  -I, --stdin-input                    read input document from stdin
  -t, --target {rego,wasm}             set the runtime to exercise (default rego)
  -u, --unknowns stringArray           set paths to treat as unknown during partial evaluation (default [input])
```

____

## opa build

Build an OPA bundle

### Synopsis

Build an OPA bundle.

The 'build' command packages OPA policy and data files into bundles. Bundles are
gzipped tarballs containing policies and data. Paths referring to directories are
loaded recursively.

    $ ls
    example.rego

    $ opa build -b .

You can load bundles into OPA on the command-line:

    $ ls
    bundle.tar.gz example.rego

    $ opa run bundle.tar.gz

You can also configure OPA to download bundles from remote HTTP endpoints:

    $ opa run --server \
        --set bundles.example.resource=bundle.tar.gz \
        --set services.example.url=http://localhost:8080

Inside another terminal in the same directory, serve the bundle via HTTP:

    $ python3 -m http.server --bind localhost 8080

For more information on bundles see https://www.openpolicyagent.org/docs/latest/management-bundles/.

### Common Flags


When -b is specified the 'build' command assumes paths refer to existing bundle files
or directories following the bundle structure. If multiple bundles are provided, their
contents are merged. If there are any merge conflicts (e.g., due to conflicting bundle
roots), the command fails. When loading an existing bundle file, the .manifest from
the input bundle will be included in the output bundle. Flags that set .manifest fields
(such as --revision) override input bundle .manifest fields.

The -O flag controls the optimization level. By default, optimization is disabled (-O=0).
When optimization is enabled the 'build' command generates a bundle that is semantically
equivalent to the input files however the structure of the files in the bundle may have
been changed by rewriting, inlining, pruning, etc. Higher optimization levels may result
in longer build times.

The 'build' command supports targets (specified by -t):

    rego    The default target emits a bundle containing a set of policy and data files
            that are semantically equivalent to the input files. If optimizations are
            disabled the output may simply contain a copy of the input policy and data
            files. If optimization is enabled at least one entrypoint must be supplied,
            either via the -e option, or via entrypoint metadata annotations.

    wasm    The wasm target emits a bundle containing a WebAssembly module compiled from
            the input files for each specified entrypoint. The bundle may contain the
            original policy or data files.

    plan    The plan target emits a bundle containing a plan, i.e., an intermediate
            representation compiled from the input files for each specified entrypoint.
            This is for further processing, OPA cannot evaluate a "plan bundle" like it
            can evaluate a wasm or rego bundle.

The -e flag tells the 'build' command which documents (entrypoints) will be queried by 
the software asking for policy decisions, so that it can focus optimization efforts and 
ensure that document is not eliminated by the optimizer.
Note: Unless the --prune-unused flag is used, any rule transitively referring to a 
package or rule declared as an entrypoint will also be enumerated as an entrypoint.

### Signing


The 'build' command can be used to verify the signature of a signed bundle and
also to generate a signature for the output bundle the command creates.

If the directory path(s) provided to the 'build' command contain a ".signatures.json" file,
it will attempt to verify the signatures included in that file. The bundle files
or directory path(s) to verify must be specified using --bundle.

For more information on the bundle signing and verification, see
https://www.openpolicyagent.org/docs/latest/management-bundles/#signing.

Example:

    $ opa build --verification-key /path/to/public_key.pem --signing-key /path/to/private_key.pem --bundle foo

Where foo has the following structure:

    foo/
      |
      +-- bar/
      |     |
      |     +-- data.json
      |
      +-- policy.rego
      |
      +-- .manifest
      |
      +-- .signatures.json


The 'build' command will verify the signatures using the public key provided by the --verification-key flag.
The default signing algorithm is RS256 and the --signing-alg flag can be used to specify
a different one. The --verification-key-id and --scope flags can be used to specify the name for the key
provided using the --verification-key flag and scope to use for bundle signature verification respectively.

If the verification succeeds, the 'build' command will write out an updated ".signatures.json" file
to the output bundle. It will use the key specified by the --signing-key flag to sign
the token in the ".signatures.json" file.

To include additional claims in the payload use the --claims-file flag to provide a JSON file
containing optional claims.

For more information on the format of the ".signatures.json" file
see https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-format.

### Capabilities


The 'build' command can validate policies against a configurable set of OPA capabilities.
The capabilities define the built-in functions and other language features that policies
may depend on. For example, the following capabilities file only permits the policy to
depend on the "plus" built-in function ('+'):

    {
        "builtins": [
            {
                "name": "plus",
                "infix": "+",
                "decl": {
                    "type": "function",
                    "args": [
                        {
                            "type": "number"
                        },
                        {
                            "type": "number"
                        }
                    ],
                    "result": {
                        "type": "number"
                    }
                }
            }
        ]
    }

Capabilities can be used to validate policies against a specific version of OPA.
The OPA repository contains a set of capabilities files for each OPA release. For example,
the following command builds a directory of policies ('./policies') and validates them
against OPA v0.22.0:

    opa build ./policies --capabilities v0.22.0


```
opa build <path> [<path> [...]] [flags]
```

### Options

```
  -b, --bundle                         load paths as bundle files or root directories
      --capabilities string            set capabilities version or capabilities.json file path
      --claims-file string             set path of JSON file containing optional claims (see: https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-format)
      --debug                          enable debug output
  -e, --entrypoint string              set slash separated entrypoint path
      --exclude-files-verify strings   set file names to exclude during bundle verification
  -h, --help                           help for build
      --ignore strings                 set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)
  -O, --optimize int                   set optimization level
  -o, --output string                  set the output filename (default "bundle.tar.gz")
      --prune-unused                   exclude dependents of entrypoints
  -r, --revision string                set output bundle revision
      --scope string                   scope to use for bundle signature verification
      --signing-alg string             name of the signing algorithm (default "RS256")
      --signing-key string             set the secret (HMAC) or path of the PEM file containing the private key (RSA and ECDSA)
      --signing-plugin string          name of the plugin to use for signing/verification (see https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-plugin
  -t, --target {rego,wasm,plan}        set the output bundle target type (default rego)
      --verification-key string        set the secret (HMAC) or path of the PEM file containing the public key (RSA and ECDSA)
      --verification-key-id string     name assigned to the verification key used for bundle verification (default "default")
```

____

## opa capabilities

Print the capabilities of OPA

### Synopsis

Show capabilities for OPA.

The 'capabilities' command prints the OPA capabilities, prior to and including the version of OPA used.

Print a list of all existing capabilities version names

    $ opa capabilities
    v0.17.0
    v0.17.1
    ...
    v0.37.1
    v0.37.2
    v0.38.0
    ...

Print the capabilities of the current version

    $ opa capabilities --current
    {
        "builtins": [...],
        "future_keywords": [...],
        "wasm_abi_versions": [...]
    }

Print the capabilities of a specific version

    $ opa capabilities --version v0.32.1
    {
        "builtins": [...],
        "future_keywords": null,
        "wasm_abi_versions": [...]
    }

Print the capabilities of a capabilities file

    $ opa capabilities --file ./capabilities/v0.32.1.json
    {
        "builtins": [...],
        "future_keywords": null,
        "wasm_abi_versions": [...]
    }



```
opa capabilities [flags]
```

### Options

```
      --current          print current capabilities
      --file string      print current capabilities
  -h, --help             help for capabilities
      --version string   print capabilities of a specific version
```

____

## opa check

Check Rego source files

### Synopsis

Check Rego source files for parse and compilation errors.
	
	If the 'check' command succeeds in parsing and compiling the source file(s), no output
	is produced. If the parsing or compiling fails, 'check' will output the errors
	and exit with a non-zero exit code.

```
opa check <path> [path [...]] [flags]
```

### Options

```
  -b, --bundle                 load paths as bundle files or root directories
      --capabilities string    set capabilities version or capabilities.json file path
  -f, --format {pretty,json}   set output format (default pretty)
  -h, --help                   help for check
      --ignore strings         set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)
  -m, --max-errors int         set the number of errors to allow before compilation fails early (default 10)
  -s, --schema string          set schema file path or directory path
  -S, --strict                 enable compiler strict mode
```

____

## opa deps

Analyze Rego query dependencies

### Synopsis

Print dependencies of provided query.

Dependencies are categorized as either base documents, which is any data loaded
from the outside world, or virtual documents, i.e values that are computed from rules.

### Example

Given a policy like this:

	package policy

	import future.keywords.if
	import future.keywords.in

	allow if is_admin

	is_admin if "admin" in input.user.roles

To evaluate the dependencies of a simple query (e.g. data.policy.allow),
we'd run opa deps like demonstrated below:

	$ opa deps --data policy.rego data.policy.allow
	+------------------+----------------------+
	|  BASE DOCUMENTS  |  VIRTUAL DOCUMENTS   |
	+------------------+----------------------+
	| input.user.roles | data.policy.allow    |
	|                  | data.policy.is_admin |
	+------------------+----------------------+

From the output we're able to determine that the allow rule depends on
the input.user.roles base document, as well as the virtual document (rule)
data.policy.is_admin.


```
opa deps <query> [flags]
```

### Options

```
  -b, --bundle string          set bundle file(s) or directory path(s). This flag can be repeated.
  -d, --data string            set policy or data file(s). This flag can be repeated.
  -f, --format {pretty,json}   set output format (default pretty)
  -h, --help                   help for deps
      --ignore strings         set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)
```

____

## opa eval

Evaluate a Rego query

### Synopsis

Evaluate a Rego query and print the result.

### Examples


To evaluate a simple query:

    $ opa eval 'x := 1; y := 2; x < y'

To evaluate a query against JSON data:

    $ opa eval --data data.json 'name := data.names[_]'

To evaluate a query against JSON data supplied with a file:// URL:

    $ opa eval --data file:///path/to/file.json 'data'


### File & Bundle Loading


The --bundle flag will load data files and Rego files contained
in the bundle specified by the path. It can be either a
compressed tar archive bundle file or a directory tree.

    $ opa eval --bundle /some/path 'data'

Where /some/path contains:

    foo/
      |
      +-- bar/
      |     |
      |     +-- data.json
      |
      +-- baz.rego
      |
      +-- manifest.yaml

The JSON file 'foo/bar/data.json' would be loaded and rooted under
'data.foo.bar' and the 'foo/baz.rego' would be loaded and rooted under the
package path contained inside the file. Only data files named data.json or
data.yaml will be loaded. In the example above the manifest.yaml would be
ignored.

See https://www.openpolicyagent.org/docs/latest/management-bundles/ for more details
on bundle directory structures.

The --data flag can be used to recursively load ALL *.rego, *.json, and
*.yaml files under the specified directory.

The -O flag controls the optimization level. By default, optimization is disabled (-O=0).
When optimization is enabled the 'eval' command generates a bundle from the files provided
with either the --bundle or --data flag. This bundle is semantically equivalent to the input
files however the structure of the files in the bundle may have been changed by rewriting, inlining,
pruning, etc. This resulting optimized bundle is used to evaluate the query. If optimization is
enabled at least one entrypoint must be supplied, either via the -e option, or via entrypoint
metadata annotations.

### Output Formats


Set the output format with the --format flag.

    --format=json      : output raw query results as JSON
    --format=values    : output line separated JSON arrays containing expression values
    --format=bindings  : output line separated JSON objects containing variable bindings
    --format=pretty    : output query results in a human-readable format
    --format=source    : output partial evaluation results in a source format
    --format=raw       : output the values from query results in a scripting friendly format

### Schema


The -s/--schema flag provides one or more JSON Schemas used to validate references to the input or data documents.
Loads a single JSON file, applying it to the input document; or all the schema files under the specified directory.

    $ opa eval --data policy.rego --input input.json --schema schema.json
    $ opa eval --data policy.rego --input input.json --schema schemas/

### Capabilities


When passing a capabilities definition file via --capabilities, one can restrict which
hosts remote schema definitions can be retrieved from. For example, a capabilities.json
containing

    {
        "builtins": [ ... ],
        "allow_net": [ "kubernetesjsonschema.dev" ]
    }

would disallow fetching remote schemas from any host but "kubernetesjsonschema.dev".
Setting allow_net to an empty array would prohibit fetching any remote schemas.

Not providing a capabilities file, or providing a file without an allow_net key, will
permit fetching remote schemas from any host.

Note that the metaschemas http://json-schema.org/draft-04/schema, http://json-schema.org/draft-06/schema,
and http://json-schema.org/draft-07/schema, are always available, even without network
access.


```
opa eval <query> [flags]
```

### Options

```
  -b, --bundle string                                     set bundle file(s) or directory path(s). This flag can be repeated.
      --capabilities string                               set capabilities version or capabilities.json file path
      --count int                                         number of times to repeat each benchmark (default 1)
      --coverage                                          report coverage
  -d, --data string                                       set policy or data file(s). This flag can be repeated.
      --disable-early-exit                                disable 'early exit' optimizations
      --disable-indexing                                  disable indexing optimizations
      --disable-inlining stringArray                      set paths of documents to exclude from inlining
  -e, --entrypoint string                                 set slash separated entrypoint path
      --explain {off,full,notes,fails,debug}              enable query explanations (default off)
      --fail                                              exits with non-zero exit code on undefined/empty result and errors
      --fail-defined                                      exits with non-zero exit code on defined/non-empty result and errors
  -f, --format {json,values,bindings,pretty,source,raw}   set output format (default json)
  -h, --help                                              help for eval
      --ignore strings                                    set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)
      --import string                                     set query import(s). This flag can be repeated.
  -i, --input string                                      set input file path
      --instrument                                        enable query instrumentation metrics (implies --metrics)
      --metrics                                           report query performance metrics
  -O, --optimize int                                      set optimization level
      --package string                                    set query package
  -p, --partial                                           perform partial evaluation
      --pretty-limit int                                  set limit after which pretty output gets truncated (default 80)
      --profile                                           perform expression profiling
      --profile-limit int                                 set number of profiling results to show (default 10)
      --profile-sort string                               set sort order of expression profiler results
  -s, --schema string                                     set schema file path or directory path
      --shallow-inlining                                  disable inlining of rules that depend on unknowns
      --show-builtin-errors                               collect and return all encountered built-in errors, built in errors are not fatal
      --stdin                                             read query from stdin
  -I, --stdin-input                                       read input document from stdin
  -S, --strict                                            enable compiler strict mode
      --strict-builtin-errors                             treat the first built-in function error encountered as fatal
  -t, --target {rego,wasm}                                set the runtime to exercise (default rego)
      --timeout duration                                  set eval timeout (default unlimited)
  -u, --unknowns stringArray                              set paths to treat as unknown during partial evaluation (default [input])
```

____

## opa exec

Execute against input files

### Synopsis

Execute against input files.

The 'exec' command executes OPA against one or more input files. If the paths
refer to directories, OPA will execute against files contained inside those
directories, recursively.

The 'exec' command accepts a --config-file/-c or series of --set options as
arguments. These options behave the same as way as 'opa run'. Since the 'exec'
command is intended to execute OPA in one-shot, the 'exec' command will
manually trigger plugins before and after policy execution:

Before: Discovery -> Bundle -> Status
After: Decision Logs

By default, the 'exec' command executes the "default decision" (specified in
the OPA configuration) against each input file. This can be overridden by
specifying the --decision argument and pointing at a specific policy decision,
e.g., opa exec --decision /foo/bar/baz ...

```
opa exec <path> [<path> [...]] [flags]
```

### Options

```
  -b, --bundle string                        set bundle file(s) or directory path(s). This flag can be repeated.
  -c, --config-file string                   set path of configuration file
      --decision string                      set decision to evaluate
      --fail                                 exits with non-zero exit code on undefined/empty result and errors
      --fail-defined                         exits with non-zero exit code on defined/non-empty result and errors
  -f, --format {pretty,json}                 set output format (default pretty)
  -h, --help                                 help for exec
      --log-format {text,json,json-pretty}   set log format (default json)
  -l, --log-level {debug,info,error}         set log level (default error)
      --log-timestamp-format string          set log timestamp format (OPA_LOG_TIMESTAMP_FORMAT environment variable)
      --set stringArray                      override config values on the command line (use commas to specify multiple values)
      --set-file stringArray                 override config values with files on the command line (use commas to specify multiple values)
```

____

## opa fmt

Format Rego source files

### Synopsis

Format Rego source files.

The 'fmt' command takes a Rego source file and outputs a reformatted version. If no file path
is provided - this tool will use stdin.
The format of the output is not defined specifically; whatever this tool outputs
is considered correct format (with the exception of bugs).

If the '-w' option is supplied, the 'fmt' command with overwrite the source file
instead of printing to stdout.

If the '-d' option is supplied, the 'fmt' command will output a diff between the
original and formatted source.

If the '-l' option is supplied, the 'fmt' command will output the names of files
that would change if formatted. The '-l' option will suppress any other output
to stdout from the 'fmt' command.

If the '--fail' option is supplied, the 'fmt' command will return a non zero exit
code if a file would be reformatted.

```
opa fmt [path [...]] [flags]
```

### Options

```
  -d, --diff    only display a diff of the changes
      --fail    non zero exit code on reformat
  -h, --help    help for fmt
  -l, --list    list all files who would change when formatted
  -w, --write   overwrite the original source file
```

____

## opa inspect

Inspect OPA bundle(s)

### Synopsis

Inspect OPA bundle(s).

The 'inspect' command provides a summary of the contents in OPA bundle(s). Bundles are
gzipped tarballs containing policies and data. The 'inspect' command reads bundle(s) and lists
the following:

* packages that are contributed by .rego files
* data locations defined by the data.json and data.yaml files
* manifest data
* signature data
* information about the Wasm module files
* package- and rule annotations

Example:

    $ ls
    bundle.tar.gz
    $ opa inspect bundle.tar.gz

You can provide exactly one OPA bundle or path to the 'inspect' command on the command-line. If you provide a path
referring to a directory, the 'inspect' command will load that path as a bundle and summarize its structure and contents.


```
opa inspect <path> [<path> [...]] [flags]
```

### Options

```
  -a, --annotations            list annotations
  -f, --format {json,pretty}   set output format (default pretty)
  -h, --help                   help for inspect
```

____

## opa parse

Parse Rego source file

### Synopsis

Parse Rego source file and print AST.

```
opa parse <path> [flags]
```

### Options

```
  -f, --format {pretty,json}   set output format (default pretty)
  -h, --help                   help for parse
      --json-include string    include or exclude optional elements. By default comments are included. Current options: locations, comments. E.g. --json-include locations,-comments will include locations and exclude comments.
```

____

## opa run

Start OPA in interactive or server mode

### Synopsis

Start an instance of the Open Policy Agent (OPA).

To run the interactive shell:

    $ opa run

To run the server:

    $ opa run -s

The 'run' command starts an instance of the OPA runtime. The OPA runtime can be
started as an interactive shell or a server.

When the runtime is started as a shell, users can define rules and evaluate
expressions interactively. When the runtime is started as a server, OPA exposes
an HTTP API for managing policies, reading and writing data, and executing
queries.

The runtime can be initialized with one or more files that contain policies or
data. If the '--bundle' option is specified the paths will be treated as policy
bundles and loaded following standard bundle conventions. The path can be a
compressed archive file or a directory which will be treated as a bundle.
Without the '--bundle' flag OPA will recursively load ALL rego, JSON, and YAML
files.

When loading from directories, only files with known extensions are considered.
The current set of file extensions that OPA will consider are:

    .json          # JSON data
    .yaml or .yml  # YAML data
    .rego          # Rego file

Non-bundle data file and directory paths can be prefixed with the desired
destination in the data document with the following syntax:

    <dotted-path>:<file-path>

To set a data file as the input document in the interactive shell use the
"repl.input" path prefix with the input file:

    repl.input:<file-path>

Example:

    $ opa run repl.input:input.json

Which will load the "input.json" file at path "data.repl.input".

Use the "help input" command in the interactive shell to see more options.


File paths can be specified as URLs to resolve ambiguity in paths containing colons:

    $ opa run file:///c:/path/to/data.json

URL paths to remote public bundles (http or https) will be parsed as shorthand
configuration equivalent of using repeated --set flags to accomplish the same:

	$ opa run -s https://example.com/bundles/bundle.tar.gz

The above shorthand command is identical to:

    $ opa run -s --set "services.cli1.url=https://example.com" \
                 --set "bundles.cli1.service=cli1" \
                 --set "bundles.cli1.resource=/bundles/bundle.tar.gz" \
                 --set "bundles.cli1.persist=true"

The 'run' command can also verify the signature of a signed bundle.
A signed bundle is a normal OPA bundle that includes a file
named ".signatures.json". For more information on signed bundles
see https://www.openpolicyagent.org/docs/latest/management-bundles/#signing.

The key to verify the signature of signed bundle can be provided
using the --verification-key flag. For example, for RSA family of algorithms,
the command expects a PEM file containing the public key.
For HMAC family of algorithms (eg. HS256), the secret can be provided
using the --verification-key flag.

The --verification-key-id flag can be used to optionally specify a name for the
key provided using the --verification-key flag.

The --signing-alg flag can be used to specify the signing algorithm.
The 'run' command uses RS256 (by default) as the signing algorithm.

The --scope flag can be used to specify the scope to use for
bundle signature verification.

Example:

    $ opa run --verification-key secret --signing-alg HS256 --bundle bundle.tar.gz

The 'run' command will read the bundle "bundle.tar.gz", check the
".signatures.json" file and perform verification using the provided key.
An error will be generated if "bundle.tar.gz" does not contain a ".signatures.json" file.
For more information on the bundle verification process see
https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-verification.

The 'run' command can ONLY be used with the --bundle flag to verify signatures
for existing bundle files or directories following the bundle structure.

To skip bundle verification, use the --skip-verify flag.


```
opa run [flags]
```

### Options

```
  -a, --addr strings                         set listening address of the server (e.g., [ip]:<port> for TCP, unix://<path> for UNIX domain socket) (default [:8181])
      --authentication {token,tls,off}       set authentication scheme (default off)
      --authorization {basic,off}            set authorization scheme (default off)
  -b, --bundle                               load paths as bundle files or root directories
  -c, --config-file string                   set path of configuration file
      --diagnostic-addr strings              set read-only diagnostic listening address of the server for /health and /metric APIs (e.g., [ip]:<port> for TCP, unix://<path> for UNIX domain socket)
      --disable-telemetry                    disables anonymous information reporting (see: https://www.openpolicyagent.org/docs/latest/privacy)
      --exclude-files-verify strings         set file names to exclude during bundle verification
  -f, --format string                        set shell output format, i.e, pretty, json (default "pretty")
      --h2c                                  enable H2C for HTTP listeners
  -h, --help                                 help for run
  -H, --history string                       set path of history file (default "$HOME/.opa_history")
      --ignore strings                       set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)
      --log-format {text,json,json-pretty}   set log format (default json)
  -l, --log-level {debug,info,error}         set log level (default info)
      --log-timestamp-format string          set log timestamp format (OPA_LOG_TIMESTAMP_FORMAT environment variable)
  -m, --max-errors int                       set the number of errors to allow before compilation fails early (default 10)
      --min-tls-version {1.0,1.1,1.2,1.3}    set minimum TLS version to be used by OPA's server (default 1.2)
      --pprof                                enables pprof endpoints
      --ready-timeout int                    wait (in seconds) for configured plugins before starting server (value <= 0 disables ready check)
      --scope string                         scope to use for bundle signature verification
  -s, --server                               start the runtime in server mode
      --set stringArray                      override config values on the command line (use commas to specify multiple values)
      --set-file stringArray                 override config values with files on the command line (use commas to specify multiple values)
      --shutdown-grace-period int            set the time (in seconds) that the server will wait to gracefully shut down (default 10)
      --shutdown-wait-period int             set the time (in seconds) that the server will wait before initiating shutdown
      --signing-alg string                   name of the signing algorithm (default "RS256")
      --skip-verify                          disables bundle signature verification
      --tls-ca-cert-file string              set path of TLS CA cert file
      --tls-cert-file string                 set path of TLS certificate file
      --tls-cert-refresh-period duration     set certificate refresh period
      --tls-private-key-file string          set path of TLS private key file
      --verification-key string              set the secret (HMAC) or path of the PEM file containing the public key (RSA and ECDSA)
      --verification-key-id string           name assigned to the verification key used for bundle verification (default "default")
  -w, --watch                                watch command line files for changes
```

____

## opa sign

Generate an OPA bundle signature

### Synopsis

Generate an OPA bundle signature.

The 'sign' command generates a digital signature for policy bundles. It generates a
".signatures.json" file that dictates which files should be included in the bundle,
what their SHA hashes are, and is cryptographically secure.

The signatures file is a JSON file with an array containing a single JSON Web Token (JWT)
that encapsulates the signature for the bundle.

The --signing-alg flag can be used to specify the algorithm to sign the token. The 'sign'
command uses RS256 (by default) as the signing algorithm.
See https://www.openpolicyagent.org/docs/latest/configuration/#keys
for a list of supported signing algorithms.

The key to be used for signing the JWT MUST be provided using the --signing-key flag.
For example, for RSA family of algorithms, the command expects a PEM file containing
the private key.
For HMAC family of algorithms (eg. HS256), the secret can be provided using
the --signing-key flag.

OPA 'sign' can ONLY be used with the --bundle flag to load paths that refer to
existing bundle files or directories following the bundle structure.

	$ opa sign --signing-key /path/to/private_key.pem --bundle foo

Where foo has the following structure:

	foo/
	  |
	  +-- bar/
	  |     |
	  |     +-- data.json
	  |
	  +-- policy.rego
	  |
	  +-- .manifest

This will create a ".signatures.json" file in the current directory.
The --output-file-path flag can be used to specify a different location for
the ".signatures.json" file.

The content of the ".signatures.json" file is shown below:

	{
	  "signatures": [
		"eyJhbGciOiJSUzI1NiJ9.eyJmaWxlcyI6W3sibmFtZSI6Ii5tYW5pZmVzdCIsImhhc2giOiIxODc0NWRlNzJjMDFlODBjZDlmNTIwZjQxOGMwMDlhYzRkMmMzZDAyYjE3YTUwZTJkMDQyMTU4YmMzNTJhMzJkIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6ImJhci9kYXRhLmpzb24iLCJoYXNoIjoiOTNhMjM5NzFhOTE0ZTVlYWNiZjBhOGQyNTE1NGNkYTMwOWMzYzFjNzJmYmI5OTE0ZDQ3YzYwZjNjYjY4MTU4OCIsImFsZ29yaXRobSI6IlNIQS0yNTYifSx7Im5hbWUiOiJwb2xpY3kucmVnbyIsImhhc2giOiJkMGYyNDJhYWUzNGRiNTRlZjU2NmJlYTRkNDVmY2YxOTcwMGM1ZDhmODdhOWRiOTMyZGZhZDZkMWYwZjI5MWFjIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9XX0.lNsmRqrmT1JI4Z_zpY6IzHRZQAU306PyOjZ6osquixPuTtdSBxgbsdKDcp7Civw3B77BgygVsvx4k3fYr8XCDKChm0uYKScrpFr9_yS6g5mVTQws3KZncZXCQHdupRFoqMS8vXAVgJr52C83AinYWABwH2RYq_B0ZPf_GDzaMgzpep9RlDNecGs57_4zlyxmP2ESU8kjfX8jAA6rYFKeGXJHMD-j4SassoYIzYRv9YkHx8F8Y2ae5Kd5M24Ql0kkvqc_4eO_T9s4nbQ4q5qGHGE-91ND1KVn2avcUyVVPc0-XCR7EH8HnHgCl0v1c7gX1RL7ET7NJbPzfmzQAzk0ZW0dEHI4KZnXSpqy8m-3zAc8kIARm2QwoNEWpy3MWiooPeZVSa9d5iw1aLrbyumfjBP0vCQEPes-Aa6PrARwd5jR9SacO5By0-4emzskvJYRZqbfJ9tXSXDMcAFOAm6kqRPJaj8AO4CyajTC_Lt32_0OLeXqYgNpt3HDqLqGjrb-8fVeQc-hKh0aES8XehQqXj4jMwfsTyj5alsXZm08LwzcFlfQZ7s1kUtmr0_BBNJYcdZUdlu6Qio3LFSRYXNuu6edAO1VH5GKqZISvE1uvDZb2E0Z-rtH-oPp1iSpfvsX47jKJ42LVpI6OahEBri44dzHOIwwm3CIuV8gFzOwR0k"
	  ]
	}

And the decoded JWT payload has the following form:

	{
	  "files": [
		{
		  "name": ".manifest",
		  "hash": "18745de72c01e80cd9f520f418c009ac4d2c3d02b17a50e2d042158bc352a32d",
		  "algorithm": "SHA-256"
		},
		{
		  "name": "policy.rego",
		  "hash": "d0f242aae34db54ef566bea4d45fcf19700c5d8f87a9db932dfad6d1f0f291ac",
		  "algorithm": "SHA-256"
		},
		{
		  "name": "bar/data.json",
		  "hash": "93a23971a914e5eacbf0a8d25154cda309c3c1c72fbb9914d47c60f3cb681588",
		  "algorithm": "SHA-256"
		}
	  ]
	}

The "files" field is generated from the files under the directory path(s)
provided to the 'sign' command. During bundle signature verification, OPA will check
each file name (ex. "foo/bar/data.json") in the "files" field
exists in the actual bundle. The file content is hashed using SHA256.

To include additional claims in the payload use the --claims-file flag to provide
a JSON file containing optional claims.

For more information on the format of the ".signatures.json" file see
https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-format.


```
opa sign <path> [<path> [...]] [flags]
```

### Options

```
  -b, --bundle                    load paths as bundle files or root directories
      --claims-file string        set path of JSON file containing optional claims (see: https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-format)
  -h, --help                      help for sign
  -o, --output-file-path string   set the location for the .signatures.json file (default ".")
      --signing-alg string        name of the signing algorithm (default "RS256")
      --signing-key string        set the secret (HMAC) or path of the PEM file containing the private key (RSA and ECDSA)
      --signing-plugin string     name of the plugin to use for signing/verification (see https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-plugin
```

____

## opa test

Execute Rego test cases

### Synopsis

Execute Rego test cases.

The 'test' command takes a file or directory path as input and executes all
test cases discovered in matching files. Test cases are rules whose names have the prefix "test_".

If the '--bundle' option is specified the paths will be treated as policy bundles
and loaded following standard bundle conventions. The path can be a compressed archive
file or a directory which will be treated as a bundle. Without the '--bundle' flag OPA
will recursively load ALL *.rego, *.json, and *.yaml files for evaluating the test cases.

Test cases under development may be prefixed "todo_" in order to skip their execution,
while still getting marked as skipped in the test results.

Example policy (example/authz.rego):

	package authz

	import future.keywords.if

	allow if {
		input.path == ["users"]
		input.method == "POST"
	}

	allow if {
		input.path == ["users", input.user_id]
		input.method == "GET"
	}

Example test (example/authz_test.rego):

	package authz_test

	import data.authz.allow

	test_post_allowed {
		allow with input as {"path": ["users"], "method": "POST"}
	}

	test_get_denied {
		not allow with input as {"path": ["users"], "method": "GET"}
	}

	test_get_user_allowed {
		allow with input as {"path": ["users", "bob"], "method": "GET", "user_id": "bob"}
	}

	test_get_another_user_denied {
		not allow with input as {"path": ["users", "bob"], "method": "GET", "user_id": "alice"}
	}

	todo_test_user_allowed_http_client_data {
		false # Remember to test this later!
	}

Example test run:

	$ opa test ./example/

If used with the '--bench' option then tests will be benchmarked.

Example benchmark run:

	$ opa test --bench ./example/

The optional "gobench" output format conforms to the Go Benchmark Data Format.


```
opa test <path> [path [...]] [flags]
```

### Options

```
      --bench                              benchmark the unit tests
      --benchmem                           report memory allocations with benchmark results (default true)
  -b, --bundle                             load paths as bundle files or root directories
      --capabilities string                set capabilities version or capabilities.json file path
      --count int                          number of times to repeat each test (default 1)
  -c, --coverage                           report coverage (overrides debug tracing)
  -z, --exit-zero-on-skipped               skipped tests return status 0
      --explain {fails,full,notes,debug}   enable query explanations (default fails)
  -f, --format {pretty,json,gobench}       set output format (default pretty)
  -h, --help                               help for test
      --ignore strings                     set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)
  -m, --max-errors int                     set the number of errors to allow before compilation fails early (default 10)
  -r, --run string                         run only test cases matching the regular expression.
  -t, --target {rego,wasm}                 set the runtime to exercise (default rego)
      --threshold float                    set coverage threshold and exit with non-zero status if coverage is less than threshold %
      --timeout duration                   set test timeout (default 5s, 30s when benchmarking)
  -v, --verbose                            set verbose reporting mode
```

____

## opa version

Print the version of OPA

### Synopsis

Show version and build information for OPA.

```
opa version [flags]
```

### Options

```
  -c, --check   check for latest OPA release
  -h, --help    help for version
```


