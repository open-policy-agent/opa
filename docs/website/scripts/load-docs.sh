#!/bin/bash

set -e

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

if [[ ${DEV} != "" ]]; then
    ALL_RELEASES=()
fi

for release in ${ALL_RELEASES}; do
    CUR_SEM_VER=${release#"v"}

    # ignore any release candidate versions, for now if they
    # are the "latest" they'll be documented under "edge"
    if [[ "${CUR_SEM_VER}" == *"rc"* ]]; then
        continue
    fi

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

    # ignore the tag if there is no corresponding OPA binary available on the GitHub Release page
    BINARY_URL=https://github.com/open-policy-agent/opa/releases/download/${release}/opa_linux_amd64
    curl_exit_code=0
    curl --silent --location --head --fail $BINARY_URL >/dev/null || curl_exit_code=$?
    if [[ $curl_exit_code -ne 0 ]]; then
        echo "WARNING: skipping $release because $BINARY_URL does not exist (or GET failed...)"
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
echo "Releases to consider: ${RELEASES[*]}"

echo "Cleaning generated folder"
rm -rf ${ROOT_DIR}/docs/website/generated/docs/*

echo "Removing data/releases.yaml file"
rm -f ${RELEASES_YAML_FILE}

mkdir -p $(dirname ${RELEASES_YAML_FILE})

if [[ ${DEV} == "" ]]; then
    echo 'Adding "latest" version to releases.yaml'
    echo "- latest" > ${RELEASES_YAML_FILE}
fi

for release in "${RELEASES[@]}"; do
    version_docs_dir=${ROOT_DIR}/docs/website/generated/docs/${release}
    mkdir -p ${version_docs_dir}

    echo "Checking out release ${release}"

    # Don't error if the checkout fails
    set +e
    git archive --format=tar ${release} content | tar x -C ${version_docs_dir} --strip-components=1
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
done

# Create the "edge" version from current working tree
echo 'Adding "edge" to releases.yaml'
echo "- edge" >> ${RELEASES_YAML_FILE}

# Link instead of copy so we don't need to re-generate each time.
# Use a relative link so it works in a container more easily.
mkdir -p ${ROOT_DIR}/docs/website/generated/docs
ln -s ../../../content ${ROOT_DIR}/docs/website/generated/docs/edge

# Create a "latest" version from the latest semver found
if [[ ${DEV} == "" ]]; then
    cp -r ${ROOT_DIR}/docs/website/generated/docs/${RELEASES[0]} ${ROOT_DIR}/docs/website/generated/docs/latest
fi
