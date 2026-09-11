#!/usr/bin/env bash
# Refreshes internal/cmd/genbuiltinmetadata/implementations.json, which records the builtins
# implemented by Rego interpreters other than this one. The builtin reference documentation uses
# it to report where each builtin is available, so users can tell whether a policy will run on
# something other than OPA itself.
#
# Two publishing shapes are supported, preferring the first:
#
#   1. A versioned capabilities directory, capabilities/<version>.json, holding one snapshot per
#      release the way this repository's own capabilities/ directory does. This carries the whole
#      history, so we can report the version each builtin arrived in.
#   2. A single capabilities.json at the repository root, read as of the latest release tag. Only
#      that release is known, so builtins are recorded without a version.
#
# Only builtin names are kept: that is all the documentation needs, and it keeps the checked-in
# diff reviewable.
#
# Requires: gh (authenticated), jq. Run build/update-implementations.sh, then `go generate` to
# fold the result into builtin_metadata.json.

set -euo pipefail

# Implementations to track: "<id>|<label>|<owner/repo>". The id is the key used in
# builtin_metadata.json, the label is what the documentation displays.
IMPLEMENTATIONS=(
    "swift|Swift|open-policy-agent/swift-opa"
    "java|Java|open-policy-agent/java-opa-sdk"
)

OUTPUT="${OUTPUT:-internal/cmd/genbuiltinmetadata/implementations.json}"
CAPABILITIES_DIR="capabilities"
CAPABILITIES_FILE="capabilities.json"

for cmd in gh jq; do
    if ! command -v "${cmd}" > /dev/null; then
        echo "error: ${cmd} is required" >&2
        exit 1
    fi
done

workdir=$(mktemp -d)
trap 'rm -rf "${workdir}"' EXIT

# Previous contents, used to carry an implementation forward when its capabilities cannot be
# resolved. Dropping it instead would silently remove a column from the documentation.
previous="${workdir}/previous.json"
if [ -f "${OUTPUT}" ]; then
    cp "${OUTPUT}" "${previous}"
else
    echo '{"implementations":[]}' > "${previous}"
fi

entries="${workdir}/entries"
mkdir -p "${entries}"

keep_previous() {
    jq --arg id "$1" '[.implementations[] | select(.id == $id)]' "${previous}" \
        > "${entries}/$1.json"
}

# fetch_json <repo> <path> <ref> <destination>: writes a file from a repository via the contents
# API. Release assets would need the download CDN, which the contents API avoids.
fetch_json() {
    local repo=$1 path=$2 ref=$3 dest=$4
    local encoded="${dest}.b64"

    if ! gh api "repos/${repo}/contents/${path}?ref=${ref}" --jq '.content' > "${encoded}" \
        2> /dev/null || [ ! -s "${encoded}" ]; then
        return 1
    fi

    base64 --decode < "${encoded}" > "${dest}"
    jq -e '.builtins | arrays and length > 0' "${dest}" > /dev/null 2>&1
}

# versioned_snapshots <repo> <ref>: lists the versions in the capabilities directory, oldest
# first, or nothing when the implementation does not publish one.
versioned_snapshots() {
    local repo=$1 ref=$2

    gh api "repos/${repo}/contents/${CAPABILITIES_DIR}?ref=${ref}" --jq '.[].name' 2> /dev/null \
        | sed -n 's/^\([0-9][0-9.]*\)\.json$/\1/p' \
        | sort -t. -k1,1n -k2,2n -k3,3n
}

for spec in "${IMPLEMENTATIONS[@]}"; do
    IFS='|' read -r id label repo <<< "${spec}"

    echo "==> ${repo}"

    if ! branch=$(gh api "repos/${repo}" --jq '.default_branch' 2> /dev/null); then
        echo "    warning: repository unavailable; keeping any existing entry" >&2
        keep_previous "${id}"
        continue
    fi

    # Every snapshot as {version, builtins}, oldest first, so the version each builtin arrived in
    # is the first one listing it.
    snapshots="${workdir}/${id}-snapshots"
    : > "${snapshots}"

    for version in $(versioned_snapshots "${repo}" "${branch}"); do
        caps="${workdir}/${id}-${version}.json"
        if ! fetch_json "${repo}" "${CAPABILITIES_DIR}/${version}.json" "${branch}" "${caps}"; then
            echo "    warning: ${CAPABILITIES_DIR}/${version}.json unreadable; skipping" >&2
            continue
        fi
        jq -n --arg version "${version}" --slurpfile caps "${caps}" \
            '{version: $version, builtins: ($caps[0].builtins | map(.name) | unique)}' \
            >> "${snapshots}"
    done

    if [ -s "${snapshots}" ]; then
        version=$(jq -rs '.[-1].version' "${snapshots}")
        echo "    ${version} (history from $(jq -rs '.[0].version' "${snapshots}")):" \
            "$(jq -rs '.[-1].builtins | length' "${snapshots}") builtins"

        # builtins maps each name to the version it first appeared in.
        jq -s \
            --arg id "${id}" \
            --arg label "${label}" \
            --arg repo "${repo}" \
            --arg version "${version}" \
            '[{
                id: $id,
                label: $label,
                repo: $repo,
                version: $version,
                builtins: (reduce .[] as $snapshot ({};
                    reduce $snapshot.builtins[] as $name (.;
                        if has($name) then . else .[$name] = $snapshot.version end)))
            }]' "${snapshots}" > "${entries}/${id}.json"
        continue
    fi

    # No versioned directory: fall back to the root capabilities file at the latest release.
    if ! tag=$(gh api "repos/${repo}/releases/latest" --jq '.tag_name' 2> /dev/null); then
        echo "    warning: no versioned ${CAPABILITIES_DIR}/ and no release; keeping entry" >&2
        keep_previous "${id}"
        continue
    fi

    caps="${workdir}/${id}-latest.json"
    if ! fetch_json "${repo}" "${CAPABILITIES_FILE}" "${tag}" "${caps}"; then
        echo "    warning: ${tag} publishes no capabilities; keeping any existing entry" >&2
        keep_previous "${id}"
        continue
    fi

    # Tags are published both as "0.0.9" and "v0.3.0" depending on the implementation, so
    # normalize away the prefix before reporting the version to the documentation.
    version="${tag#v}"
    echo "    ${version} (no history): $(jq '.builtins | length' "${caps}") builtins"

    # Only one release is known, so no builtin can be attributed to a version.
    jq -n \
        --arg id "${id}" \
        --arg label "${label}" \
        --arg repo "${repo}" \
        --arg version "${version}" \
        --slurpfile caps "${caps}" \
        '[{
            id: $id,
            label: $label,
            repo: $repo,
            version: $version,
            builtins: ($caps[0].builtins | map({key: .name, value: null}) | from_entries)
        }]' > "${entries}/${id}.json"
done

# Emit implementations in the order they are configured above, so the output is stable.
ordered="${workdir}/ordered"
: > "${ordered}"
for spec in "${IMPLEMENTATIONS[@]}"; do
    IFS='|' read -r id _ _ <<< "${spec}"
    cat "${entries}/${id}.json" >> "${ordered}"
done

jq -s '{implementations: add}' "${ordered}" > "${OUTPUT}"

echo "wrote ${OUTPUT}"
