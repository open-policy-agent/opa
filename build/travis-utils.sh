# Travis-CI sets TRAVIS_PULL_REQUEST=false when the build is triggered for
# changes pushed into github.com/open-policy-agent/opa.
function is_travis_push_env() {
    if [ "$TRAVIS_PULL_REQUEST" = "false" ]; then
        return 0
    fi
    return 1
}

# Travis-CI sets TRAVIS_TAG=<tag> when the build is triggered for a tag.
# If the tag matches the source version then we can assume this is build is
# for a release.
function is_travis_release_env() {
  if [ "$TRAVIS_TAG" = "v$(make version)" ]; then
    return 0
  fi
  return 1
}
