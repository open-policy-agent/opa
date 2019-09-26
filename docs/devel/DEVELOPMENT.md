## Development

OPA is written in the [Go](https://golang.org) programming language.

If you are not familiar with Go we recommend you read through the [How to Write Go
Code](https://golang.org/doc/code.html) article to familiarize yourself with the standard Go development environment.

Requirements:

- Git
- GitHub account (if you are contributing)
- Go (version 1.12 is supported though older versions are likely to work)
- GNU Make

## Getting Started

After cloning the repository, just run `make`. This will:

- Install required dependencies, e.g., the parser-generator ("pigeon").
- Build the OPA binary.
- Run all of the tests.
- Run all of the static analysis checks.

If `make` fails with `main.go:20: running "pigeon": exec: "pigeon":
executable file not found in $PATH` make sure that `$GOPATH/bin` is
in `$PATH`. If `$GOPATH` is undefined, it defaults to
`$HOME/go/bin`:

```
export PATH=$PATH:$GOPATH/bin

# OR

export PATH=$PATH:$HOME/go/bin
```

If the build was successful, a binary will be produced in the top directory (`opa_<OS>_<ARCH>`).

Verify the build was successful with `./opa_<OS>_<ARCH> run`.

You can re-build the project with `make build`, execute all of the tests
with `make test`, and execute all of the performance benchmarks with `make perf`.

The static analysis checks (e.g., `go fmt`, `golint`, `go vet`) can be run
with `make check`.

> To correct any imports or style errors run `make fmt`.

## Workflow

1. Go to [https://github.com/open-policy-agent/opa](https://github.com/open-policy-agent/opa) and fork the repository
   into your account by clicking the "Fork" button.

1. Clone the fork to your local machine.
    
    ```bash
    # Note: With Go modules this repo can be in _any_ location,
    # and does not need to be in the GOSRC path.
    git clone git@github.com/<GITHUB USERNAME>/opa.git opa
    cd opa
    git remote add upstream https://github.com/open-policy-agent/opa.git
    ```

1. Create a branch for your changes.

    ```
    git checkout -b somefeature
    ```

1. Update your local branch with upstream.

    ```
    git fetch upstream
    git rebase upstream/master
    ```

1. Develop your changes and regularly update your local branch against upstream.

    - Be sure to run `make check` before submitting your pull request. You
      may need to run `go fmt` on your code to make it comply with standard Go
      style.

1. Commit changes and push to your fork.

    ```
    git commit -s
    git push origin somefeature
    ```

1. Submit a Pull Request via https://github.com/\<GITHUB USERNAME>/opa. You
   should be prompted to with a "Compare and Pull Request" button that
   mentions your branch.

1. Once your Pull Request has been reviewed and signed off please squash your
   commits. If you have a specific reason to leave multiple commits in the
   Pull Request, please mention it in the discussion.

   > If you are not familiar with squashing commits, see [the following blog post for a good overview](http://gitready.com/advanced/2009/02/10/squashing-commits-with-rebase.html).

## Benchmarks

Several packages in this repository implement benchmark tests. To execute the
benchmarks you can run `make perf` in the top-level directory. We use the Go
benchmarking framework for all benchmarks. The benchmarks run on every pull
request.

To help catch performance regressions we also run a batch job that compares the
benchmark results from the tip of master against the last major release. All of
the results are posted and can be viewed
[here](https://opa-benchmark-results.s3.amazonaws.com/index.html).

## Dependencies

OPA is a Go module [https://github.com/golang/go/wiki/Modules](https://github.com/golang/go/wiki/Modules)
and dependencies are tracked with the standard [go.mod](../../go.mod) file.

We also keep a full copy of the dependencies in the [vendor](../../vendor)
directory. All `go` commands from the [Makefile](../../Makefile) will enable
module mode by setting `GO111MODULE=on GOFLAGS=-mod=vendor` which will also
force using the `vendor` directory.

To update a dependency ensure that `GO111MODULE` is either on, or the repository
qualifies for `auto` to enable module mode. Then simply use `go get ..` to get
the version desired. This should update the [go.mod](../../go.mod) and (potentially)
[go.sum](../../go.sum) files. After this you *MUST* run `go mod vendor` to ensure
that the `vendor` directory is in sync.

After updating dependencies, be sure to check if the parser-generator ("pigeon")
was updated. If it was, re-generate the parser and commit the changes.

Example workflow for updating a dependency:

```bash
go get -u github.com/sirupsen/logrus@v1.4.2  # Get the specified version of the package.
go mod tidy                                  # (Somewhat optional) Prunes removed dependencies.
go mod vendor                                # Ensure the vendor directory is up to date.
```

If dependencies have been removed ensure to run `go mod tidy` to clean them up.


### Tool Dependencies

We use some tools such as `pigeon`, `goimports`, etc which are versioned and vendored
with OPA as depedencies. See [tools.go](../../tools.go) for a list of tools.

More details on the pattern: [https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md](https://github.com/go-modules-by-example/index/blob/master/010_tools/README.md)

Update these the same way as any other Go package. Ensure that any build script
only uses `go run ./vendor/<tool pkg>` to force using the correct version.

## Rego

If you need to modify the Rego syntax you must update ast/rego.peg. Both `make
build` and `make test` will re-generate the parser but if you want to test the
parser generation explicitly you can run `make generate`.

If you are modifying the Rego syntax you must commit the parser source file
(ast/parser.go) that `make generate` produces when you are done. The generated
code is kept in the repository so that commands such as `go get` work.

## Go

If you need to update the version of Go used to build OPA you must update these
files in the root of this repository:

* `Makefile`- which is used to produce releases locally. Update the `GOVERSION` variable.
