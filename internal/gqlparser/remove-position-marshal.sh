#!/usr/bin/env bash
set -e

# Relies on perl to do in-place regex search-and-replace for the Position annotations
# Relies on find to recursively enumerate all the Go files.

# Add Position json annotation
# Position information is pruned by pruneIrrelevantGraphQLASTNodes() later anyway
for f in $(find . -name "*.go"); do perl -pi -e 's/\*Position \`dump:"-"\`/\*Position \`dump:"-" json:"-"\`/g' $f; done
