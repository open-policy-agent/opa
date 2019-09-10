#!/bin/bash

set -xe

ORIGINAL_COMMIT=$(git symbolic-ref -q --short HEAD || git name-rev --name-only HEAD)
# If no name can be found "git name-rev" returns
# "undefined", in which case we'll just use the
# current commit ID.
if [[ "${ORIGINAL_COMMIT}" == "undefined" ]]; then
    ORIGINAL_COMMIT=$(git rev-parse HEAD)
fi

ROOT_DIR=$(git rev-parse --show-toplevel)
RELEASES_YAML_FILE=${ROOT_DIR}/docs/website/data/releases.yaml
GIT_VERSION=$(git --version)

# Look at the git tags and generate a list of releases
# that we want to show docs for.
if [[ -z ${OFFLINE} ]]; then
    git fetch --tags ${REPOSITORY_URL:-https://github.com/open-policy-agent/opa.git}
fi
ALL_RELEASES=$(git tag -l | sort -r -V)
RELEASES=()
PREV_MAJOR_VER="-1"
PREV_MINOR_VER="-1"
for release in ${ALL_RELEASES}; do
    CUR_SEM_VER=${release#"v"}
    SEMVER_REGEX='[^0-9]*\([0-9]*\)[.]\([0-9]*\)[.]\([0-9]*\)\([0-9A-Za-z-]*\)'
    CUR_MAJOR_VER=$(echo ${CUR_SEM_VER} | sed -e "s#${SEMVER_REGEX}#\1#")
    CUR_MINOR_VER=$(echo ${CUR_SEM_VER} | sed -e "s#${SEMVER_REGEX}#\2#")
    CUR_PATCH_VER=$(echo ${CUR_SEM_VER} | sed -e "s#${SEMVER_REGEX}#\3#")

    # ignore versions from before we used this static site generator
    if [[ (${CUR_MAJOR_VER} -lt 0) || \
            (${CUR_MAJOR_VER} -le 0 && ${CUR_MINOR_VER} -lt 11) || \
            (${CUR_MAJOR_VER} -le 0 && ${CUR_MINOR_VER} -le 10 && ${CUR_PATCH_VER} -le 7) ]]; then
        continue
    fi

    # The releases are sorted in order by semver from newest to oldest, and we only want
    # the latest point release for each minor version
    if [[ "${CUR_MAJOR_VER}" != "${PREV_MAJOR_VER}" || \
             ("${CUR_MAJOR_VER}" = "${PREV_MAJOR_VER}" && \
                "${CUR_MINOR_VER}" != "${PREV_MINOR_VER}") ]]; then
        RELEASES+=(${release})
    fi

    PREV_MAJOR_VER=${CUR_MAJOR_VER}
    PREV_MINOR_VER=${CUR_MINOR_VER}
done

echo "Git version: ${GIT_VERSION}"

echo "Saving current workspace state"
STASH_TOKEN=$(cat /dev/urandom | LC_CTYPE=C tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
git stash push --include-untracked -m "${STASH_TOKEN}"

function restore_tree {
    echo "Returning to commit ${ORIGINAL_COMMIT}"
    git checkout ${ORIGINAL_COMMIT}

    # Only pop from the stash if we had stashed something earlier
    if [[ -n "$(git stash list | head -1 | grep ${STASH_TOKEN} || echo '')" ]]; then
        git stash pop
    fi
}

function cleanup {
    EXIT_CODE=$?

    if [[ "${EXIT_CODE}" != "0" ]]; then 
        # on errors attempt to restore the starting tree state
        restore_tree

        echo "Error loading docs"
        exit ${EXIT_CODE}
    fi

    echo "Docs loading complete"
}

trap cleanup EXIT

echo "Cleaning generated folder"
rm -rf ${ROOT_DIR}/docs/website/generated/*

echo "Removing data/releases.yaml file"
rm -f ${RELEASES_YAML_FILE}

mkdir -p $(dirname ${RELEASES_YAML_FILE})

echo 'Adding "latest" version to releases.yaml'
echo "- latest" > ${RELEASES_YAML_FILE}

for release in "${RELEASES[@]}"; do
    version_docs_dir=${ROOT_DIR}/docs/website/generated/docs/${release}

    mkdir -p ${version_docs_dir}

    echo "Checking out release ${release}"

    # Don't error if the checkout fails
    set +e
    git checkout ${release}
    errc=$?
    set -e

    # only add the version to the releases.yaml data file
    # if we were able to check out the version, otherwise skip it..
    if [[ "${errc}" == "0" ]]; then
        echo "Adding ${release} to releases.yaml"
        echo "- ${release}" >> ${RELEASES_YAML_FILE}
    else
        echo "WARNING: Failed to check out version ${version}!!"
    fi

    echo "Copying doc content from tag ${release}"
    cp -r ${ROOT_DIR}/docs/content/* ${version_docs_dir}/

done

# Go back to the original tree state
restore_tree

# Create the "edge" version from current working tree
echo 'Adding "edge" to releases.yaml'
echo "- edge" >> ${RELEASES_YAML_FILE}

# Link instead of copy so we don't need to re-generate each time.
# Use a relative link so it works in a container more easily.
ln -s ../../../content ${ROOT_DIR}/docs/website/generated/docs/edge

# Create a "latest" version from the latest semver found
ln -s ${ROOT_DIR}/docs/website/generated/docs/${RELEASES[0]} ${ROOT_DIR}/docs/website/generated/docs/latest
