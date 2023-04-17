#!/bin/bash
set -eo pipefail

wasm-opt --print-call-graph $1 |
  awk -F\" '/\/\/ call/{ print $2 "," $4 }'
