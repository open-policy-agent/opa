---
layout: docs
title: Development
section: references
sort_order: 100
---

## Development

OPA is written in the [Go](https://golang.org) programming language.

If you are not familiar with Go we recommend you read through the [How to Write Go
Code](https://golang.org/doc/code.html) article to familiarize yourself with the standard Go development environment.

Requirements:

- Git
- GitHub account (if you are contributing)
- Go (version 1.11 is supported though older versions are likely to work)
- GNU Make

## Getting Started

After cloning the repository, just run `make`. This will:

- Install required dependencies, e.g., the parser-generator ("pigeon").
- Build the OPA binary.
- Run all of the tests.
- Run all of the static analysis checks.

If the build was successful, a binary will be produced in the top directory (`opa_<OS>_<ARCH>`).

Verify the build was successful with `./opa_<OS>_<ARCH> run`.

You can re-build the project with `make build`, execute all of the tests
with `make test`, and execute all of the performance benchmarks with `make perf`.

The static analysis checks (e.g., `go fmt`, `golint`, `go vet`) can be run
with `make check`.

## Workflow

1. Go to [https://github.com/open-policy-agent/opa](https://github.com/open-policy-agent/opa) and fork the repository
   into your account by clicking the "Fork" button.

1. Clone the fork to your local machine.

    ```
    cd $GOPATH
    mkdir -p src/github.com/open-policy-agent
    cd src/github.com/open-policy-agent
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

## Dependencies

[Glide](https://github.com/Masterminds/glide) is a command line tool used for
dependency management. You must have Glide installed in order to add new
dependencies or update existing dependencies. If you are not changing
dependencies you do not have to install Glide, all of the dependencies are
contained in the vendor directory.

Update `glide.yaml` if you are adding a new dependency and then run:

```
glide update --strip-vendor
```

This assumes you have Glide v0.12 or newer installed.

After updating dependencies, be sure to check if the parser-generator ("pigeon")
was updated. If it was, re-generate the parser and commit the changes.

## Rego

If you need to modify the Rego syntax you must update ast/rego.peg. Both `make
build` and `make test` will re-generate the parser but if you want to test the
parser generation explicitly you can run `make generate`.

If you are modifying the Rego syntax you must commit the parser source file
(ast/parser.go) that `make generate` produces when you are done. The generated
code is kept in the repository so that commands such as `go get` work.

## Go

If you need to update the version of Go that OPA's CI and release
builder use, update the Go version in the Makefile and .travis.yml
files. You will also need to bump the version of the release builder
and re-build it using the `release-builder` target. Note, you will
need push access on the openpolicyagent DockerHub account to release
the new version of the `release-builder` as this is not automated yet.
