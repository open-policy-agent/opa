package runtime

import (
	"embed"
	"fmt"
	"reflect"
	"testing"
)

//go:embed internal/fixtures
var fixtures embed.FS

func TestInitPolicy(t *testing.T) {
	testCases := map[string]struct {
		PolicyDir      embed.FS
		PolicyPath     string
		VersionData    map[string]interface{}
		ExpectedResult initPolicyResult
	}{
		"empty policy dir, no empty result": {
			PolicyDir:  embed.FS{},
			PolicyPath: ".",
			VersionData: map[string]interface{}{
				"version":      "0.48.0",
				"go_version":   "1.19.0",
				"platform":     "linux/arm64",
				"image":        "official",
				"image_flavor": "",
				"vcs":          "",
				"timestamp":    "2023-01-10 18:31:22",
				"hostname":     "example.com",
			},
			ExpectedResult: initPolicyResult{},
		},
		"simple message policy": {
			PolicyDir:  fixtures,
			PolicyPath: "internal/fixtures/init_policies/simple",
			VersionData: map[string]interface{}{
				"version":      "0.48.0",
				"go_version":   "1.19.0",
				"platform":     "linux/arm64",
				"image":        "official",
				"image_flavor": "",
				"vcs":          "",
				"timestamp":    "2023-01-10 18:31:22",
				"hostname":     "example.com",
			},
			ExpectedResult: initPolicyResult{
				Messages: []string{"old version"},
			},
		},
		"dismissible error policy": {
			PolicyDir:  fixtures,
			PolicyPath: "internal/fixtures/init_policies/error",
			VersionData: map[string]interface{}{
				"version":      "0.1.0",
				"go_version":   "1.19.0",
				"platform":     "linux/arm64",
				"image":        "official",
				"image_flavor": "",
				"vcs":          "",
				"timestamp":    "2023-01-10 18:31:22",
				"hostname":     "example.com",
				"env":          map[string]string{},
			},
			ExpectedResult: initPolicyResult{
				DismissibleErrors: []dismissibleError{
					{
						Error:         "version is deprecated, please upgrade or set OPA_VERSION_DEPRECATED to start this deprecated OPA",
						DismissEnvVar: "OPA_VERSION_DEPRECATED",
						Dismissed:     false,
					},
				},
			},
		},
		"dismissible error policy, when dismissed by env": {
			PolicyDir:  fixtures,
			PolicyPath: "internal/fixtures/init_policies/error",
			VersionData: map[string]interface{}{
				"version":      "0.1.0",
				"go_version":   "1.19.0",
				"platform":     "linux/arm64",
				"image":        "official",
				"image_flavor": "",
				"vcs":          "",
				"timestamp":    "2023-01-10 18:31:22",
				"hostname":     "example.com",
				"env": map[string]string{
					"OPA_VERSION_DEPRECATED": "true",
				},
			},
			ExpectedResult: initPolicyResult{
				DismissibleErrors: []dismissibleError{
					{
						Error:         "version is deprecated, please upgrade or set OPA_VERSION_DEPRECATED to start this deprecated OPA",
						DismissEnvVar: "OPA_VERSION_DEPRECATED",
						Dismissed:     true,
					},
				},
			},
		},
		"fatal error policy": {
			PolicyDir:  fixtures,
			PolicyPath: "internal/fixtures/init_policies/fatal",
			VersionData: map[string]interface{}{
				"version":      "0.1.0",
				"go_version":   "1.19.0",
				"platform":     "linux/arm64",
				"image":        "official",
				"image_flavor": "",
				"vcs":          "",
				"timestamp":    "2023-01-10 18:31:22",
				"hostname":     "example.com",
			},
			ExpectedResult: initPolicyResult{
				FatalErrors: []error{
					fmt.Errorf("version is insecure and not safe for production use, please upgrade"),
				},
			},
		},
		"combination policy": {
			PolicyDir:  fixtures,
			PolicyPath: "internal/fixtures/init_policies/combination",
			VersionData: map[string]interface{}{
				"version":      "0.1.0",
				"go_version":   "1.19.0",
				"platform":     "darwin/arm64",
				"image":        "unofficial",
				"image_flavor": "",
				"vcs":          "",
				"timestamp":    "2023-01-10 18:31:22",
				"hostname":     "example.com",
				"env":          map[string]string{},
			},
			ExpectedResult: initPolicyResult{
				Messages: []string{
					"this is our best OPA yet",
				},
				DismissibleErrors: []dismissibleError{
					{
						Error:         "version is deprecated, please upgrade or set OPA_VERSION_DEPRECATED to start this deprecated OPA",
						DismissEnvVar: "OPA_VERSION_DEPRECATED",
					},
				},
				FatalErrors: []error{
					fmt.Errorf("unofficial build"),
				},
			},
		},
	}

	for testCaseName, testCase := range testCases {
		t.Run(testCaseName, func(t *testing.T) {
			result, err := evalInitPolicy(testCase.PolicyDir, testCase.PolicyPath, testCase.VersionData)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(testCase.ExpectedResult, result) {
				t.Fatalf("expected\n%#v\nbut got\n%#v", testCase.ExpectedResult, result)
			}
		})
	}
}
