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
    version_docs_dir=${ROOT_DIR}/docs/generated/docs/v${release}
    mkdir -p ${version_docs_dir}

    echo "Copying the documentation for release v${release}"
    echo "- ${release}" >> ${RELEASES_YAML_FILE}

    cp ${ROOT_DIR}/docs/content/docs/* ${version_docs_dir}/
done

git checkout ${CURRENT_BRANCH}
