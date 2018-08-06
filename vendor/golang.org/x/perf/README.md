# Go performance measurement, storage, and analysis tools

This subrepository holds the source for various packages and tools
related to performance measurement, storage, and analysis.

[cmd/benchstat](cmd/benchstat) contains a command-line tool that
computes and compares statistics about benchmarks.

[cmd/benchsave](cmd/benchsave) contains a command-line tool for
publishing benchmark results.

[storage](storage) contains the https://perfdata.golang.org/ benchmark
result storage system.

[analysis](analysis) contains the https://perf.golang.org/ benchmark
result analysis system.

Both storage and analysis can be run locally; the following commands will run
the complete stack on your machine with an in-memory datastore.

```
go get -u golang.org/x/perf/storage/localperfdata
go get -u golang.org/x/perf/analysis/localperf
localperfdata -addr=:8081 -view_url_base=http://localhost:8080/search?q=upload: &
localperf -addr=:8080 -storage=http://localhost:8081
```

The storage system is designed to have a [standardized
API](https://perfdata.golang.org/), and we encourage additional analysis
tools to be written against the API. A client can be found in the
[storage](https://godoc.org/golang.org/x/perf/storage) package.

## Download/Install

The easiest way to install is to run `go get -u golang.org/x/perf/cmd/...`. You can
also manually git clone the repository to `$GOPATH/src/golang.org/x/perf`.

## Report Issues / Send Patches

This repository uses Gerrit for code changes. To learn how to submit changes to
this repository, see https://golang.org/doc/contribute.html.

The main issue tracker for the perf repository is located at
https://github.com/golang/go/issues. Prefix your issue with "x/perf:" in the
subject line, so it is easy to find.
