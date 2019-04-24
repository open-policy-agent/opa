#!/bin/bash

set -xe

RELEASES=$(cat RELEASES)

ORIGINAL_COMMIT=$(git rev-parse --abbrev-ref HEAD)
ROOT_DIR=$(git rev-parse --show-toplevel)
RELEASES_YAML_FILE=${ROOT_DIR}/docs/data/releases.yaml
GIT_VERSION=$(git --version)

echo "Git version: ${GIT_VERSION}"

echo "Saving current workspace state"
STASH_TOKEN=$(uuidgen)
git stash push --include-untracked -m "${STASH_TOKEN}"

function cleanup {
    EXIT_CODE=$?

    echo "Returning to commit ${ORIGINAL_COMMIT}"
    git checkout ${ORIGINAL_COMMIT}

    # Only pop from the stash if we had stashed something earlier
    if [[ -n "$(git stash list | head -1 | grep ${STASH_TOKEN} || echo '')" ]]; then
        git stash pop
    fi

    if [[ "${EXIT_CODE}" != "0" ]]; then 
        echo "Error loading docs"
        exit ${EXIT_CODE}
    fi

    echo "Docs loading complete"
}

trap cleanup EXIT

echo "Cleaning generated folder"
rm -rf ${ROOT_DIR}/docs/generated/*

echo "Removing data/releases.yaml file"
rm -f ${RELEASES_YAML_FILE}

for release in ${RELEASES}; do
    version_docs_dir=${ROOT_DIR}/docs/generated/docs/${release}
    mkdir -p ${version_docs_dir}

    echo "Adding ${release} to releases.yaml"
    echo "- ${release}" >> ${RELEASES_YAML_FILE}

    echo "Checking out tag ${release}"
    git checkout ${release}

    echo "Copying ${ROOT_DIR}/docs/content/docs/* to ${version_docs_dir}/"
    cp ${ROOT_DIR}/docs/content/docs/* ${version_docs_dir}/
done


