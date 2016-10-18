#!/usr/bin/env bash

set -e

DEFAULT_N=10

# This is the number of commits to run the benchmark on.
N=${1:-$DEFAULT_N}

HEAD=$(git rev-parse HEAD)
EPOCH=$(git rev-list $HEAD | tail -n 1)

echo "commit,subject,author,timestamp,benchmark,runs,latency(ns)"

for rev in $(git rev-list ${EPOCH}..${HEAD} | head -n $N | xargs); do
    # Check out next revision to run benchmarks on.
    git checkout $rev 1>&2 2>/dev/null
    metadata=$(git log --format="%h,%s,%ae,%at" -1 $rev)

    # Run performance benchmark on revision. Output results, one benchmark per
    # line with revision metadata included.
    make perf 2>/dev/null | \
        grep "^Benchmark" | \
        awk -v metadata="$metadata" '{print metadata "," $1 "," $2 "," $3 }'
done
