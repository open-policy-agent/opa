// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/open-policy-agent/opa/util/test"
)

func setTestEnvVar(t *testing.T, name, value string) string {
	envKey := fmt.Sprintf("%s_%s", t.Name(), name)
	t.Setenv(envKey, value)
	return envKey
}

func TestSubEnvVarsVarsSubOne(t *testing.T) {
	envKey := setTestEnvVar(t, "var1", "foo")
	configYaml := fmt.Sprintf("field1: ${%s}", envKey)

	expected := "field1: foo"

	actual := subEnvVars(configYaml)

	if actual != expected {
		t.Errorf("Expected: '%s'\nActual: '%s'", expected, actual)
	}
}

func TestSubEnvVarsVarsSubMulti(t *testing.T) {
	urlEnvKey := setTestEnvVar(t, "SERVICE_URL", "https://example.com/control-plane-api/v1")
	tokenEnvKey := setTestEnvVar(t, "BEARER_TOKEN", "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm")
	configYaml := fmt.Sprintf(`
	services:
	- name: acmecorp
		url: ${%s}
		credentials:
		bearer:
			token: "${%s}"

	discovery:
	name: /example/discovery
	prefix: configuration`, urlEnvKey, tokenEnvKey)

	expected := `
	services:
	- name: acmecorp
		url: https://example.com/control-plane-api/v1
		credentials:
		bearer:
			token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

	discovery:
	name: /example/discovery
	prefix: configuration`

	actual := subEnvVars(configYaml)

	if actual != expected {
		t.Errorf("\nExpected: '%s'\nActual: '%s'", expected, actual)
	}
}

func TestSubEnvVarsVarsNoVars(t *testing.T) {
	configYaml := "field1: foo"
	expected := "field1: foo"

	actual := subEnvVars(configYaml)

	if actual != expected {
		t.Errorf("Expected: '%s'\nActual: '%s'", expected, actual)
	}
}

func TestSubEnvVarsVarsEmptyString(t *testing.T) {
	configYaml := ""
	expected := ""

	actual := subEnvVars(configYaml)

	if actual != expected {
		t.Errorf("Expected: '%s'\nActual: '%s'", expected, actual)
	}
}

func TestSubEnvVarsVarsSubMissingEnvVar(t *testing.T) {
	envKey := setTestEnvVar(t, "var1", "foo")
	configYaml := fmt.Sprintf("field1: '${%s}'", envKey)

	// Remove the env var and expect the system to sub in ""
	os.Unsetenv(envKey)
	expected := "field1: ''"

	actual := subEnvVars(configYaml)

	if actual != expected {
		t.Errorf("Expected: '%s'\nActual: '%s'", expected, actual)
	}
}

func TestSubEnvVarsVarsSubEmptyVarName(t *testing.T) {
	configYaml := "field1: '${}'"
	expected := "field1: ''"

	actual := subEnvVars(configYaml)

	if actual != expected {
		t.Errorf("Expected: '%s'\nActual: '%s'", expected, actual)
	}
}

func TestMergeValuesNoOverride(t *testing.T) {
	dest := map[string]interface{}{}
	src := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "foo",
		},
	}

	actual := mergeValues(dest, src)

	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "foo",
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("merged map does not match expected:\n\nExpected: %+v\nActual: %+v", expected, actual)
	}
}

func TestMergeValuesOverrideSingle(t *testing.T) {
	dest := map[string]interface{}{
		"a": "bar",
	}
	src := map[string]interface{}{
		"a": "override-value",
	}

	actual := mergeValues(dest, src)

	expected := map[string]interface{}{
		"a": "override-value",
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("merged map does not match expected:\n\nExpected: %+v\nActual: %+v", expected, actual)
	}
}

func TestMergeValuesOverrideSingleNested(t *testing.T) {
	dest := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "foo",
		},
	}
	src := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "override-value",
		},
	}

	actual := mergeValues(dest, src)

	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "override-value",
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("merged map does not match expected:\n\nExpected: %+v\nActual: %+v", expected, actual)
	}
}

func TestMergeValuesOverrideMultipleNested(t *testing.T) {
	dest := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
				"k4": "v4",
			},
		},
	}
	src := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"k1": "v1-override",
				"k4": "v4-override",
			},
		},
	}

	actual := mergeValues(dest, src)

	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"k1": "v1-override",
				"k2": "v2",
				"k3": "v3",
				"k4": "v4-override",
			},
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("merged map does not match expected:\n\nExpected: %+v\nActual: %+v", expected, actual)
	}
}

func TestMergeValuesOverrideSingleList(t *testing.T) {
	dest := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []map[string]interface{}{
				{
					"k1": "v1",
					"k2": "v2",
				},
			},
		},
	}
	src := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []map[string]interface{}{
				{
					"k3": "v3",
				},
			},
		},
	}

	actual := mergeValues(dest, src)

	// The list index 0 should have been replaced instead of merging the sub objects
	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []map[string]interface{}{
				{
					"k3": "v3",
				},
			},
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("merged map does not match expected:\n\nExpected: %+v\nActual: %+v", expected, actual)
	}
}

func TestMergeValuesNoSrc(t *testing.T) {
	dest := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "foo",
		},
	}
	src := map[string]interface{}{}

	actual := mergeValues(dest, src)

	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "foo",
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("merged map does not match expected:\n\nExpected: %+v\nActual: %+v", expected, actual)
	}
}

func TestMergeValuesNoSrcOrDest(t *testing.T) {
	dest := map[string]interface{}{}
	src := map[string]interface{}{}

	actual := mergeValues(dest, src)

	expected := map[string]interface{}{}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("merged map does not match expected:\n\nExpected: %+v\nActual: %+v", expected, actual)
	}
}

func TestLoadConfigWithParamOverride(t *testing.T) {
	fs := map[string]string{"/some/config.yaml": `
services:
  acmecorp:
    url: https://example.com/control-plane-api/v1

discovery:
  name: /example/discovery
  prefix: configuration
`}

	test.WithTempFS(fs, func(rootDir string) {
		configFile := filepath.Join(rootDir, "/some/config.yaml")
		configOverrides := []string{"services.acmecorp.credentials.bearer.token=bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"}

		configBytes, err := Load(configFile, configOverrides, nil)
		if err != nil {
			t.Errorf("unexpected error loading config: " + err.Error())
		}

		config := map[string]interface{}{}
		err = yaml.Unmarshal(configBytes, &config)
		if err != nil {
			t.Errorf("unexpected error unmarshalling config")
		}

		expected := map[string]interface{}{
			"services": map[string]interface{}{
				"acmecorp": map[string]interface{}{
					"url": "https://example.com/control-plane-api/v1",
					"credentials": map[string]interface{}{
						"bearer": map[string]interface{}{
							"token": "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
						},
					},
				},
			},
			"discovery": map[string]interface{}{
				"name":   "/example/discovery",
				"prefix": "configuration",
			},
		}

		if !reflect.DeepEqual(config, expected) {
			t.Errorf("config does not match expected:\n\nExpected: %+v\nActual: %+v", expected, config)
		}
	})
}

func TestLoadConfigWithFileOverride(t *testing.T) {
	fs := map[string]string{
		"/some/config.yaml": `
services:
  acmecorp:
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "XXXXXXXXXX"

discovery:
  name: /example/discovery
  prefix: configuration
`,
		"/some/secret.txt": "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
	}

	test.WithTempFS(fs, func(rootDir string) {
		configFile := filepath.Join(rootDir, "/some/config.yaml")
		secretFile := filepath.Join(rootDir, "/some/secret.txt")
		overrideFiles := []string{fmt.Sprintf("services.acmecorp.credentials.bearer.token=%s", secretFile)}

		configBytes, err := Load(configFile, nil, overrideFiles)
		if err != nil {
			t.Errorf("unexpected error loading config: " + err.Error())
		}

		config := map[string]interface{}{}
		err = yaml.Unmarshal(configBytes, &config)
		if err != nil {
			t.Errorf("unexpected error unmarshalling config")
		}

		expected := map[string]interface{}{
			"services": map[string]interface{}{
				"acmecorp": map[string]interface{}{
					"url": "https://example.com/control-plane-api/v1",
					"credentials": map[string]interface{}{
						"bearer": map[string]interface{}{
							"token": "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
						},
					},
				},
			},
			"discovery": map[string]interface{}{
				"name":   "/example/discovery",
				"prefix": "configuration",
			},
		}

		if !reflect.DeepEqual(config, expected) {
			t.Errorf("config does not match expected:\n\nExpected: %+v\nActual: %+v", expected, config)
		}
	})
}

func TestLoadConfigWithParamOverrideNoConfigFile(t *testing.T) {
	configOverrides := []string{
		"services.acmecorp.url=https://example.com/control-plane-api/v1",
		"services.acmecorp.credentials.bearer.token=bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
		"discovery.name=/example/discovery",
		"discovery.prefix=configuration",
	}

	configBytes, err := Load("", configOverrides, nil)
	if err != nil {
		t.Errorf("unexpected error loading config: " + err.Error())
	}

	config := map[string]interface{}{}
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		t.Errorf("unexpected error unmarshalling config")
	}

	expected := map[string]interface{}{
		"services": map[string]interface{}{
			"acmecorp": map[string]interface{}{
				"url": "https://example.com/control-plane-api/v1",
				"credentials": map[string]interface{}{
					"bearer": map[string]interface{}{
						"token": "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm",
					},
				},
			},
		},
		"discovery": map[string]interface{}{
			"name":   "/example/discovery",
			"prefix": "configuration",
		},
	}

	if !reflect.DeepEqual(config, expected) {
		t.Errorf("config does not match expected:\n\nExpected: %+v\nActual: %+v", expected, config)
	}
}

func TestLoadConfigWithParamOverrideNoConfigFileWithEmptyObject(t *testing.T) {
	configOverrides := []string{
		"services.acmecorp.url=https://example.com/control-plane-api/v1",
		"services.acmecorp.headers=null",
		"services.acmecorp.credentials.s3_signing.environment_credentials=null",
		"decision_logs.plugin=my_plugin",
		"plugins.my_plugin=null",
	}

	configBytes, err := Load("", configOverrides, nil)
	if err != nil {
		t.Errorf("unexpected error loading config: " + err.Error())
	}

	config := map[string]interface{}{}
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		t.Errorf("unexpected error unmarshalling config")
	}

	expected := map[string]interface{}{
		"services": map[string]interface{}{
			"acmecorp": map[string]interface{}{
				"url":     "https://example.com/control-plane-api/v1",
				"headers": map[string]interface{}{},
				"credentials": map[string]interface{}{
					"s3_signing": map[string]interface{}{
						"environment_credentials": map[string]interface{}{},
					},
				},
			},
		},
		"decision_logs": map[string]interface{}{
			"plugin": "my_plugin",
		},
		"plugins": map[string]interface{}{
			"my_plugin": map[string]interface{}{},
		},
	}

	if !reflect.DeepEqual(config, expected) {
		t.Errorf("config does not match expected:\n\nExpected: %+v\nActual: %+v", expected, config)
	}
}
