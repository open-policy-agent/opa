#!/usr/bin/env bash

set -e

BASE_URL='http://localhost:8181/v1'

function main() {

    trap 'kill $(jobs -p)' EXIT

    # setup for examples below
    start_opa
    load_policies

    # execute each of the examples from the reference
    list_policies
    get_a_policy
    get_a_policy_source
    create_or_update_a_policy
    delete_a_policy
    get_a_document
    get_a_document_with_input
    patch_a_document
    execute_a_query
    error_format
    trace_event
}

function list_policies() {
    echo "### List Policies"
    echo ""
    curl $BASE_URL/policies -s -v
    echo ""
}

function get_a_policy() {
    echo "### Get a Policy"
    echo ""
    curl $BASE_URL/policies/example1 -s -v
    echo ""
}

function get_a_policy_source() {
    echo "#### Example Request For Source"
    echo ""
    curl $BASE_URL/policies/example1?source=true -s -v
    echo ""
}

function create_or_update_a_policy() {
    echo "### Create or Update a Policy"
    echo ""
    curl $BASE_URL/policies/example1 -X PUT -s -v --data-binary @example1.rego
    echo ""
}

function delete_a_policy() {
    echo "### Delete a Policy"
    echo ""
    curl $BASE_URL/policies/example2 -X DELETE -s -v
    echo ""

    # Re-create the policy module so that remaining reference document sections can be generated.
    curl $BASE_URL/policies/example2 -X PUT -s -v --data-binary @example2.rego >/dev/null 2>&1
}

function get_a_document() {
    # Create policy module for this example.
    curl $BASE_URL/policies/example3 -X PUT -s -v --data-binary @example3.rego >/dev/null 2>&1
    curl $BASE_URL/policies/example4 -X PUT -s -v --data-binary @example4.rego >/dev/null 2>&1
    curl $BASE_URL/data/containers -X PUT -s -v --data-binary @containers.json >/dev/null 2>&1
    echo ""

    echo "### Get a Document"
    echo ""
    curl $BASE_URL/data/opa/examples/public_servers?pretty=true -s -v
    echo ""
    curl $BASE_URL/data/opa/examples/allow_container?pretty=true -s -v -G --data-urlencode 'input=container:data.containers[container_index]'
    echo ""

    # Delete policy modules created above.
    curl $BASE_URL/policies/example3 -X DELETE -s >/dev/null 2>&1
    curl $BASE_URL/policies/example4 -X DELETE -s >/dev/null 2>&1
}

function get_a_document_with_input() {
    # Create policy module for this example.
    curl $BASE_URL/policies/example3 -X PUT -s -v --data-binary @example3.rego >/dev/null 2>&1

    echo "### Get a Document With Input"
    echo ""
    echo "#### Example Request"
    echo ""
    curl $BASE_URL/data/opa/examples/allow_request?pretty=true -s -v \
        -d '{"input": {"example": {"flag": true}}}' -H 'Content-Type: application: json' | jq
    echo ""
    echo "#### Example Request"
    echo ""
    curl $BASE_URL/data/opa/examples/allow_request?pretty=true -s -v \
        -d '{"input": {"example": {"flag": false}}}' -H 'Content-Type: application: json' | jq
    echo ""

    # Delete policy module created above.
    curl $BASE_URL/policies/example3 -X DELETE -s >/dev/null 2>&1
}

function patch_a_document() {
    echo "### Patch a Document"
    echo ""
    curl $BASE_URL/data/servers -X PATCH -s -v --data-binary @example-patch.json
    echo ""
}

function execute_a_query() {
    echo "### Execute a Query"
    echo ""
    curl $BASE_URL/query?pretty=true -s -v -G --data-urlencode 'q=data.servers[i].ports[_] = "p2", data.servers[i].name = name'
    echo ""
}

function error_format() {

    echo "## Errors"
    echo ""
    curl $BASE_URL/policies/missing_id -s
}

function trace_event() {

    echo "#### Trace Event Example"
    curl "$BASE_URL/query?pretty=true&explain=full" -s -v -G --data-urlencode 'q=x = "hello", x = y' | jq '.explanation[2]'
    echo ""
}

function start_opa() {
    opa run -s example.json &
    while [ 1 ]; do
        rc=0
        curl $BASE_URL/policies -s >/dev/null 2>&1 || rc=$?
        if [ $rc -eq 0 ]; then
            break
        else
            sleep 0.1
        fi
    done
}

function load_policies() {

    curl $BASE_URL/policies/example1 \
        -X PUT \
        --data-binary @example1.rego -s >/dev/null 2>&1

    curl $BASE_URL/policies/example2 \
        -X PUT \
        --data-binary @example2.rego -s >/dev/null 2>&1
}

main
