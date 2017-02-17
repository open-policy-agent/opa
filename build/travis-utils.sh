# Travis-CI sets TRAVIS_PULL_REQUEST=false when the build is triggered for
# changes pushed into github.com/open-policy-agent/opa.
function is_travis_push_env() {
    if [ "$TRAVIS_PULL_REQUEST" = "false" ]; then
        return 0
    fi
    return 1
}
