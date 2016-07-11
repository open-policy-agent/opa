#!/usr/bin/env bash

set -e

# This is the number of commits to run the benchmark on.
N=10

# This is the commit that performance benchmarks were first added.
# This script runs the performance benchmarks on the last N commits
# between HEAD and EPOCH.
EPOCH=52b6684a8c03efe51cd4e166b8b60e5197cb747d

echo "commit,subject,author,timestamp,benchmark,runs,latency(ns)"

for rev in $(git rev-list HEAD...${EPOCH} | head -n ${N} | xargs); do

    # Check out next revision to run benchmarks on.
    git checkout $rev 1>&2 2>/dev/null
    metadata=$(git log --format="%h,%s,%ae,%at" -1 $rev)

    # Run performance benchmark on revision. Output results, one benchmark per
    # line with revision metadata included.
    make perf 2>/dev/null | \
        grep "^Benchmark" | \
        awk -v metadata="$metadata" '{print metadata "," $1 "," $2 "," $3 }'
done
