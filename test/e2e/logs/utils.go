package logs

import (
	v1 "github.com/open-policy-agent/opa/v1/test/e2e/logs"
)

// GeneratePolicy generates a policy for use in Decision Log e2e tests. The
// `ruleCounts` determine how many total rules to generate, and the `ruleHits`
// are the number of them that will be evaluated. This is keyed off of
// the `input.hit` boolean value.
func GeneratePolicy(ruleCounts int, ruleHits int) string {
	return v1.GeneratePolicy(ruleCounts, ruleHits)
}

// TestLogServer implements the decision log endpoint for e2e testing.
type TestLogServer = v1.TestLogServer
