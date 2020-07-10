#!/usr/bin/env bash

usage() {
    echo "time-bound.sh <time> -- <command>"
}

TIMEOUT=${1}
shift

trap 'kill -INT -$PID' INT
timeout --signal int ${TIMEOUT} "${@}" &
PID=$!
wait $PID
RC=$?

# It's expected for the command to have timed out.
if [[ $RC == 124 ]]; then
  echo ""
  echo "Successfully stopped after ${TIMEOUT} seconds"
  exit 0
fi

exit $RC
