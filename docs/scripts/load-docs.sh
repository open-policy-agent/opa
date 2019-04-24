#!/bin/bash

set -x

RELEASES=$(cat RELEASES)

ORIGINAL_COMMIT=$(git name-rev --name-only HEAD)
# If no name can be found "git name-rev" returns
# "undefined", in which case we'll just use the
# current commit ID.
if [[ "${ORIGINAL_COMMIT}" == "undefined" ]]; then
    ORIGINAL_COMMIT=$(git rev-parse HEAD)
fi

ROOT_DIR=$(git rev-parse --show-toplevel)
RELEASES_YAML_FILE=${ROOT_DIR}/docs/data/releases.yaml
GIT_VERSION=$(git --version)

echo "Git version: ${GIT_VERSION}"

echo "Removing data/releases.yaml file"
rm -f ${RELEASES_YAML_FILE}

echo "Removing any already-copied versioned docs from the generated folder"
rm -rf ${ROOT_DIR}/docs/generated/docs/v*

for release in ${RELEASES}; do
    version_docs_dir=${ROOT_DIR}/docs/generated/docs/v${release}
    mkdir -p ${version_docs_dir}

    echo "Adding v${release} to releases.yaml"
    echo "- ${release}" >> ${RELEASES_YAML_FILE}

    echo "Checking out tag v${release}"
    git checkout v${release}

    echo "Copying ${ROOT_DIR}/docs/content/docs/* to ${version_docs_dir}/"
    cp ${ROOT_DIR}/docs/content/docs/* ${version_docs_dir}/
done

echo "Returning to commit ${ORIGINAL_COMMIT}"
git checkout ${ORIGINAL_COMMIT}

echo "Docs loading complete"
