package runtime

import (
	"context"
	"embed"
	"fmt"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/version"
	"os"
	"path/filepath"
	"strings"
)

//go:embed internal/init_policy
var initPolicyCode embed.FS

// executeInitPolicy is used in opa run to determine if the OPA instance
// complies with the init policy; if the OPA should start; and if
// there are any messages to display
func executeInitPolicy(rt *Runtime) {

	environ := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.Split(e, "=")
		if len(parts) == 2 {
			environ[parts[0]] = parts[1]
		}
	}

	result, err := evalInitPolicy(
		initPolicyCode,
		"internal/init_policy",
		map[string]interface{}{
			"version":      version.Version,
			"go_version":   version.GoVersion,
			"platform":     version.Platform,
			"image":        version.Image,
			"image_flavor": version.ImageFlavor,
			"vcs":          version.Vcs,
			"timestamp":    version.Timestamp,
			"hostname":     version.Hostname,
			"env":          environ,
		},
	)
	if err != nil {
		rt.logger.Error("init policy failed, this is a bug: %s", err.Error())
		os.Exit(1)
	}

	for _, m := range result.Messages {
		rt.logger.Warn("init policy (message): %s", m)
	}

	canStart := true
	for _, e := range result.DismissibleErrors {
		if e.Dismissed {
			rt.logger.Error("init policy (error dismissed with %s): %s", e.DismissEnvVar, e.Error)
			continue
		}
		rt.logger.Error("init policy (error not dismissed with %s): %s", e.DismissEnvVar, e.Error)
		canStart = false
	}

	for _, e := range result.FatalErrors {
		rt.logger.Error("init policy (fatal): %s", e.Error())
		canStart = false
	}

	if !canStart {
		os.Exit(1)
	}
}

type dismissibleError struct {
	// Error is the message to show to the user on start up
	Error string

	// DismissEnvVar, if this var is set then the error is not fatal
	DismissEnvVar string

	// Dismissed, is this is set, the error is considered to have been
	// dismissed and is returned for information purposes
	Dismissed bool
}

type initPolicyResult struct {
	Messages          []string
	DismissibleErrors []dismissibleError
	FatalErrors       []error
}

// resultFromExpression is a function that handles the operation of loading
// a initPolicyResult from an Rego Expression value. Rules to translate from init policy
// code and Results are encoded here
func resultFromExpression(exp *rego.ExpressionValue) (initPolicyResult, error) {
	var result initPolicyResult

	mapValue, ok := exp.Value.(map[string]interface{})
	if !ok {
		return result, fmt.Errorf("input policy expression was in unexpected format")
	}

	messages, ok := mapValue["messages"].([]interface{})
	if ok {
		for _, m := range messages {
			msg, ok := m.(string)
			if ok {
				result.Messages = append(result.Messages, msg)
			}
		}
	}

	errors, ok := mapValue["errors"].([]interface{})
	if ok {
		for _, m := range errors {
			errorData, ok := m.(map[string]interface{})
			if ok {
				message, ok := errorData["message"].(string)
				if !ok {
					return result, fmt.Errorf("init policy error message field was not present or incorrect type: %v", errorData)
				}
				dismissVar, ok := errorData["var"].(string)
				if !ok {
					return result, fmt.Errorf("init policy error var field was not present or incorrect type: %v", errorData)
				}
				dismissed, ok := errorData["dismissed"].(bool)
				if !ok {
					return result, fmt.Errorf("init policy error dismissed field was not present or incorrect type: %v", errorData)
				}

				result.DismissibleErrors = append(result.DismissibleErrors, dismissibleError{
					Error:         message,
					DismissEnvVar: dismissVar,
					Dismissed:     dismissed,
				})
			}
		}
	}

	fatals, ok := mapValue["fatals"].([]interface{})
	if ok {
		for _, m := range fatals {
			errorMsg, ok := m.(string)
			if ok {
				result.FatalErrors = append(result.FatalErrors, fmt.Errorf(errorMsg))
			}
		}
	}

	return result, nil
}

func evalInitPolicy(policyFS embed.FS, policyFSBasePath string, input map[string]interface{}) (initPolicyResult, error) {
	var result initPolicyResult
	var err error

	entries, err := policyFS.ReadDir(policyFSBasePath)
	if err != nil {
		return result, fmt.Errorf("failed to read init policy dir: %w", err)
	}

	regoArgs := []func(*rego.Rego){
		rego.Query("data.init"),
		rego.Input(input),
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := policyFS.ReadFile(filepath.Join(policyFSBasePath, entry.Name()))
		if err != nil {
			return result, fmt.Errorf("failed to read policy file at %s: %w", entry.Name(), err)
		}
		regoArgs = append(regoArgs, rego.Module(entry.Name(), string(data)))
	}

	r := rego.New(regoArgs...)

	results, err := r.Eval(context.Background())
	if err != nil {
		return result, fmt.Errorf("failed to evaluate policy: %w", err)
	}

	if len(results) == 0 {
		return initPolicyResult{}, nil
	}

	if exp, act := 1, len(results); act != exp {
		return initPolicyResult{}, fmt.Errorf("init policy returned an unexpected number of results, exp %d, got %d", exp, act)
	}

	if exp, act := 1, len(results[0].Expressions); act != exp {
		return initPolicyResult{}, fmt.Errorf("init policy returned an unexpected number of expressions, exp %d, got %d", exp, act)
	}

	result, err = resultFromExpression(results[0].Expressions[0])
	if err != nil {
		return result, fmt.Errorf("failed to build result from expression: %w", err)
	}

	return result, nil
}
