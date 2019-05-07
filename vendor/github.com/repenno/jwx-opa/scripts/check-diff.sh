#!/bin/bash

UNTRACKED=$(git ls-files --others --exclude-standard)
DIFF=$(git diff)

st=0
if [ ! -z "$DIFF" ]; then
    echo "==== START OF DIFF FOUND ==="
    echo ""
    echo "$DIFF"
    echo ""
    echo "Above diff was found."
    echo ""
    echo "==== END OF DIFF FOUND ==="
    echo ""
    st=1
fi

if [ ! -z "$UNTRACKED" ]; then
    echo "==== START OF UNTRACKED FILES FOUND ==="
    echo ""
    echo "$UNTRACKED"
    echo ""
    echo "Above untracked files were found."
    echo ""
    echo "==== END OF UNTRACKED FILES FOUND ==="
    echo ""
    st=1
fi

exit $st