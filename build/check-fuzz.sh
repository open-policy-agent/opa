#!/usr/bin/env bash

set -e

OPA_DIR=$(dirname "${BASH_SOURCE}")/..

usage() {
    echo "check-fuzz.sh <time limit>"
}

DURATION=$1

if [[ -z "${DURATION}" ]]; then
    usage
    exit 1
fi


"${OPA_DIR}"/build/time-bound.sh "${DURATION}" make -C "${OPA_DIR}" fuzz

WORKDIR=${OPA_DIR}/build/fuzzer/workdir
CRASHER_DIR=${WORKDIR}/crashers

if [[ ! -d "${WORKDIR}" ]]; then
    echo "Missing go-fuzz workdir!"
    exit 1
fi

CRASHERS=$(find "${CRASHER_DIR}" -name "*.output" | wc -l | tr -d '[:space:]')

echo ""
echo "Found ${CRASHERS} crashers!"

if [[ "${CRASHERS}" == "0" ]]; then
    exit 0
else
    echo "See ${CRASHER_DIR} for details"
    exit 1
fi
