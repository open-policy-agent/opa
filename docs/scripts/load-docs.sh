#!/bin/bash

RELEASES=$(cat RELEASES)
LATEST=$(head -n 1 RELEASES)

echo ${LATEST}

CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
ROOT_DIR=$(git rev-parse --show-toplevel)
RELEASES_YAML_FILE=${ROOT_DIR}/docs/data/releases.yaml

# Clean up releases.yaml file
rm -f ${RELEASES_YAML_FILE}

# Clean up any already-copied versioned docs
rm -rf ${ROOT_DIR}/docs/content/docs/v*

for release in ${RELEASES}; do
    echo "Copying the documentation for release v${release}"
    echo "- ${release}" >> ${RELEASES_YAML_FILE}

    mkdir ${ROOT_DIR}/docs/content/docs/v${LATEST}
    cp ${ROOT_DIR}/docs/content/docs/* ${ROOT_DIR}/docs/content/docs/v${LATEST}/
done

git checkout ${CURRENT_BRANCH}
