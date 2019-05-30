#!/usr/bin/env bash

set -e

function interrupt {
    echo "caught interrupt: exiting"
    docker kill $container_name >/dev/null
    exit 1
}

trap interrupt SIGINT SIGTERM

# Generate the test tarball from the asset files.
go run test/wasm/cmd/testgen.go \
   --input-dir test/wasm/assets \
   --output _test/testcases.tar.gz \
   $@

# Execute wasm tests inside a node container.
container_name=opa-wasm-test-$RANDOM

docker run \
       --name $container_name \
       --rm \
       -e VERBOSE=$VERBOSE \
       -v $PWD:/src \
       -w /src \
       node:8 \
       sh -c 'mkdir -p /test; tar xzf _test/testcases.tar.gz -C /test; cd /test; node test.js opa.wasm' &

# Wait for the docker run process to finish.
docker_run_pid=$!
wait $docker_run_pid
