// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"

	"github.com/open-policy-agent/opa/internal/strvals"
)

func loadConfig(params Params) ([]byte, error) {
	baseConf := map[string]interface{}{}

	// User specified config file
	if params.ConfigFile != "" {
		var bytes []byte
		var err error
		bytes, err = ioutil.ReadFile(params.ConfigFile)
		if err != nil {
			return nil, err
		}

		processedConf := subEnvVars(string(bytes))

		if err := yaml.Unmarshal([]byte(processedConf), &baseConf); err != nil {
			return []byte{}, fmt.Errorf("failed to parse %s: %s", params.ConfigFile, err)
		}
	}

	overrideConf := map[string]interface{}{}

	// User specified a config override via --set
	for _, override := range params.ConfigOverrides {
		processedOverride := subEnvVars(override)
		if err := strvals.ParseInto(processedOverride, overrideConf); err != nil {
			return []byte{}, fmt.Errorf("failed parsing --set data: %s", err)
		}
	}

	// User specified a config override value via --set-file
	for _, override := range params.ConfigOverrideFiles {
		reader := func(rs []rune) (interface{}, error) {
			bytes, err := ioutil.ReadFile(string(rs))
			value := strings.TrimSpace(string(bytes))
			return value, err
		}
		if err := strvals.ParseIntoFile(override, overrideConf, reader); err != nil {
			return []byte{}, fmt.Errorf("failed parsing --set-file data: %s", err)
		}
	}

	// Merge together base config file and overrides, prefer the overrides
	conf := mergeValues(baseConf, overrideConf)

	// Take the patched config and marshal back to YAML
	return yaml.Marshal(conf)
}

// regex looking for ${...} notation strings
var envRegex = regexp.MustCompile(`(?U:\${.*})`)

// subEnvVars will look for any environment variables in the passed in string
// with the syntax of ${VAR_NAME} and replace that string with ENV[VAR_NAME]
func subEnvVars(s string) string {
	updatedConfig := envRegex.ReplaceAllStringFunc(s, func(s string) string {
		// Trim off the '${' and '}'
		if len(s) <= 3 {
			// This should never happen..
			return ""
		}
		varName := s[2 : len(s)-1]

		// Lookup the variable in the environment. We play by
		// bash rules.. if its undefined we'll treat it as an
		// empty string instead of raising an error.
		return os.Getenv(varName)
	})

	return updatedConfig
}

// mergeValues will merge source and destination map, preferring values from the source map
func mergeValues(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			dest[k] = v
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := dest[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			dest[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		dest[k] = mergeValues(destMap, nextMap)
	}
	return dest
}
