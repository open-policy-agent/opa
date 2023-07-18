package deprecation

import (
	"testing"
)

func TestMessageForWarnings(t *testing.T) {
	testCases := map[string]struct {
		Env             []string
		Command         string
		Warnings        []warning
		ExpectedMessage string
		ExpectedFatal   bool
	}{
		"warning that does not fire": {
			Env:     []string{"OPA_FOOBAR=1"},
			Command: "foobar",
			Warnings: []warning{
				{
					MatchEnv: func(env []string) bool {
						return false
					},
					MatchCommand: func(command string) bool {
						return false
					},
					Fatal:   true,
					Message: "fatal warning",
				},
			},
			ExpectedFatal:   false,
			ExpectedMessage: "",
		},
		"warning that fires": {
			Env:     []string{"OPA_FOOBAR=1"},
			Command: "foobar",
			Warnings: []warning{
				{
					MatchEnv: func(env []string) bool {
						for _, e := range env {
							if e == "OPA_FOOBAR=1" {
								return true
							}
						}
						return false
					},
					MatchCommand: func(command string) bool {
						return command == "foobar"
					},
					Fatal:   true,
					Message: "fatal warning for foobar",
				},
			},
			ExpectedMessage: `################################################################################
###                        FATAL DEPRECATION WARNINGS                        ###
################################################################################
fatal warning for foobar
################################################################################
###                      END FATAL DEPRECATION WARNINGS                      ###
################################################################################
`,
			ExpectedFatal: true,
		},
		"two warnings that fire, one fatally": {
			Env:     []string{"OPA_FOOBAR=1"},
			Command: "foobar",
			Warnings: []warning{
				{
					MatchEnv: func(env []string) bool {
						for _, e := range env {
							if e == "OPA_FOOBAR=1" {
								return true
							}
						}
						return false
					},
					MatchCommand: func(command string) bool {
						return command == "foobar"
					},
					Fatal:   true,
					Message: "fatal warning for foobar",
				},
				{
					MatchEnv: func(env []string) bool {
						return true
					},
					MatchCommand: func(command string) bool {
						return command == "foobar"
					},
					Fatal:   false,
					Message: "non fatal warning for foobar",
				},
			},
			ExpectedMessage: `################################################################################
###                        FATAL DEPRECATION WARNINGS                        ###
################################################################################
fatal warning for foobar
--------------------------------------------------------------------------------
non fatal warning for foobar
################################################################################
###                      END FATAL DEPRECATION WARNINGS                      ###
################################################################################
`,
			ExpectedFatal: true,
		},
		"two warnings that fire, neither fatally": {
			Env:     []string{"OPA_FOOBAR=1"},
			Command: "foobar",
			Warnings: []warning{
				{
					MatchEnv: func(env []string) bool {
						for _, e := range env {
							if e == "OPA_FOOBAR=1" {
								return true
							}
						}
						return false
					},
					MatchCommand: func(command string) bool {
						return command == "foobar"
					},
					Fatal:   false,
					Message: "warning for foobar",
				},
				{
					MatchEnv: func(env []string) bool {
						return true
					},
					MatchCommand: func(command string) bool {
						return command == "foobar"
					},
					Fatal:   false,
					Message: "another warning for foobar",
				},
			},
			ExpectedMessage: `################################################################################
###                           DEPRECATION WARNINGS                           ###
################################################################################
warning for foobar
--------------------------------------------------------------------------------
another warning for foobar
################################################################################
###                         END DEPRECATION WARNINGS                         ###
################################################################################
`,
			ExpectedFatal: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			message, fatal := messageForWarnings(tc.Warnings, tc.Env, tc.Command)

			if fatal != tc.ExpectedFatal {
				t.Errorf("Expected fatal to be %v but got %v", tc.ExpectedFatal, fatal)
			}

			if message != tc.ExpectedMessage {
				t.Errorf("Expected message\n%s\nbut got\n%s", tc.ExpectedMessage, message)
			}

		})
	}
}
