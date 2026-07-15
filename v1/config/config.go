// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package config implements OPA configuration file parsing and validation.
package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/internal/ref"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
)

// ServerConfig represents the different server configuration options.
type ServerConfig struct {
	Metrics json.RawMessage `json:"metrics,omitempty"`

	Encoding json.RawMessage `json:"encoding,omitempty"`
	Decoding json.RawMessage `json:"decoding,omitempty"`

	LoggerPlugin *string `json:"logger_plugin,omitempty"`
}

// Clone creates a deep copy of ServerConfig.
func (s *ServerConfig) Clone() *ServerConfig {
	if s == nil {
		return nil
	}

	clone := &ServerConfig{}

	if s.Encoding != nil {
		clone.Encoding = make(json.RawMessage, len(s.Encoding))
		copy(clone.Encoding, s.Encoding)
	}
	if s.Decoding != nil {
		clone.Decoding = make(json.RawMessage, len(s.Decoding))
		copy(clone.Decoding, s.Decoding)
	}
	if s.Metrics != nil {
		clone.Metrics = make(json.RawMessage, len(s.Metrics))
		copy(clone.Metrics, s.Metrics)
	}
	if s.LoggerPlugin != nil {
		pluginName := *s.LoggerPlugin
		clone.LoggerPlugin = &pluginName
	}

	return clone
}

// StorageConfig represents Config's storage options.
type StorageConfig struct {
	Disk json.RawMessage `json:"disk,omitempty"`
}

// Clone creates a deep copy of StorageConfig.
func (s *StorageConfig) Clone() *StorageConfig {
	if s == nil {
		return nil
	}

	clone := &StorageConfig{}

	if s.Disk != nil {
		clone.Disk = make(json.RawMessage, len(s.Disk))
		copy(clone.Disk, s.Disk)
	}

	return clone
}

// Config represents the configuration file that OPA can be started with.
type Config struct {
	Services                     json.RawMessage            `json:"services,omitempty"`
	Labels                       map[string]string          `json:"labels,omitempty"`
	Discovery                    json.RawMessage            `json:"discovery,omitempty"`
	Bundle                       json.RawMessage            `json:"bundle,omitempty"` // Deprecated: Use `bundles` instead
	Bundles                      json.RawMessage            `json:"bundles,omitempty"`
	DecisionLogs                 json.RawMessage            `json:"decision_logs,omitempty"`
	Status                       json.RawMessage            `json:"status,omitempty"`
	Plugins                      map[string]json.RawMessage `json:"plugins,omitempty"`
	Keys                         json.RawMessage            `json:"keys,omitempty"`
	DefaultDecision              *string                    `json:"default_decision,omitempty"`
	DefaultAuthorizationDecision *string                    `json:"default_authorization_decision,omitempty"`
	Caching                      json.RawMessage            `json:"caching,omitempty"`
	NDBuiltinCache               bool                       `json:"nd_builtin_cache,omitempty"`
	PersistenceDirectory         *string                    `json:"persistence_directory,omitempty"`
	DistributedTracing           json.RawMessage            `json:"distributed_tracing,omitempty"`
	MetricsExport                json.RawMessage            `json:"metrics_export,omitempty"`
	Server                       *ServerConfig              `json:"server,omitempty"`
	Storage                      *StorageConfig             `json:"storage,omitempty"`
	Extra                        map[string]json.RawMessage `json:"-"`

	// Warnings holds non-fatal messages from config validation (e.g.
	// unrecognized options), for callers to surface to the user.
	Warnings []string `json:"-"`
}

// ParseConfig returns a valid Config object with defaults injected. The id
// and version parameters will be set in the labels map.
//
// The raw configuration is run through the embedded validation policy (see
// validate.rego): defaults are injected, fatal errors are returned, and warnings
// are attached to the returned Config.
func ParseConfig(raw []byte, id string) (*Config, error) {
	var rawConfig any
	if err := util.Unmarshal(raw, &rawConfig); err != nil {
		return nil, err
	}
	// An absent (or null) configuration is treated as an empty object.
	if rawConfig == nil {
		rawConfig = map[string]any{}
	}

	processed, warnings, err := evaluateConfigPolicy(context.TODO(), rawConfig, id)
	if err != nil {
		return nil, err
	}

	// Build from the original bytes so unchanged sections keep their exact
	// encoding, then overlay only the top-level keys the policy actually changed
	// (core defaults, or defaults injected by registered plugin policies).
	result, err := unmarshalConfig(raw)
	if err != nil {
		return nil, err
	}
	if err := result.overlayChanged(raw, processed); err != nil {
		return nil, err
	}
	result.Warnings = warnings

	// Residual checks the policy can't express: decision paths must parse.
	if _, err := ref.ParseDataPath(*result.DefaultDecision); err != nil {
		return nil, err
	}
	if _, err := ref.ParseDataPath(*result.DefaultAuthorizationDecision); err != nil {
		return nil, err
	}

	return result, nil
}

// overlayChanged applies the policy output onto the Config, but only for the
// top-level keys whose value the policy changed (or added) relative to the
// original config. Unchanged keys are left as parsed from the original bytes so
// their json.RawMessage encoding is preserved exactly.
func (c *Config) overlayChanged(raw []byte, processed map[string]any) error {
	var orig map[string]json.RawMessage
	if len(raw) > 0 {
		if err := util.Unmarshal(raw, &orig); err != nil {
			return err
		}
	}

	knownFields := knownConfigFields(reflect.ValueOf(c).Elem())
	for key, pv := range processed {
		if origBytes, ok := orig[key]; ok && jsonEqual(origBytes, pv) {
			continue // unchanged
		}
		chunk, err := json.Marshal(pv)
		if err != nil {
			return err
		}
		if field, found := knownFields[key]; found {
			field.Set(reflect.Zero(field.Type())) // replace, don't merge
			if err := util.Unmarshal(chunk, field.Addr().Interface()); err != nil {
				return err
			}
		} else {
			if c.Extra == nil {
				c.Extra = map[string]json.RawMessage{}
			}
			c.Extra[key] = chunk
		}
	}
	return nil
}

// jsonEqual reports whether raw JSON bytes and a decoded value are semantically
// equal, comparing their canonical encodings.
func jsonEqual(rawValue json.RawMessage, value any) bool {
	var decoded any
	if err := util.Unmarshal(rawValue, &decoded); err != nil {
		return false
	}
	a, err1 := json.Marshal(decoded)
	b, err2 := json.Marshal(value)
	return err1 == nil && err2 == nil && bytes.Equal(a, b)
}

// unmarshalConfig decodes JSON config bytes into a Config, routing known
// top-level keys into struct fields and keeping the rest in Extra.
func unmarshalConfig(raw []byte) (*Config, error) {
	// NOTE(sr): based on https://stackoverflow.com/a/33499066/993018
	var result Config
	knownFields := knownConfigFields(reflect.ValueOf(&result).Elem())

	if err := util.Unmarshal(raw, &result.Extra); err != nil {
		return nil, err
	}

	for key, chunk := range result.Extra {
		if field, found := knownFields[key]; found {
			if err := util.Unmarshal(chunk, field.Addr().Interface()); err != nil {
				return nil, err
			}
			delete(result.Extra, key)
		}
	}
	if len(result.Extra) == 0 {
		result.Extra = nil
	}
	return &result, nil
}

// knownConfigFields maps each JSON key to its struct field, skipping json:"-".
func knownConfigFields(objValue reflect.Value) map[string]reflect.Value {
	knownFields := map[string]reflect.Value{}
	for i := 0; i != objValue.NumField(); i++ {
		jsonName := strings.Split(objValue.Type().Field(i).Tag.Get("json"), ",")[0]
		if jsonName == "" || jsonName == "-" {
			continue
		}
		knownFields[jsonName] = objValue.Field(i)
	}
	return knownFields
}

// PluginNames returns a sorted list of names of enabled plugins.
func (c Config) PluginNames() (result []string) {
	if c.Bundle != nil || c.Bundles != nil {
		result = append(result, "bundles")
	}
	if c.Status != nil {
		result = append(result, "status")
	}
	if c.DecisionLogs != nil {
		result = append(result, "decision_logs")
	}
	for name := range c.Plugins {
		result = append(result, name)
	}
	slices.Sort(result)
	return result
}

// PluginsEnabled returns true if one or more plugin features are enabled.
//
// Deprecated: Use PluginNames instead.
func (c Config) PluginsEnabled() bool {
	return c.Bundle != nil || c.Bundles != nil || c.DecisionLogs != nil || c.Status != nil || len(c.Plugins) > 0
}

// DefaultDecisionRef returns the default decision as a reference.
func (c Config) DefaultDecisionRef() ast.Ref {
	r, _ := ref.ParseDataPath(*c.DefaultDecision)
	return r
}

// DefaultAuthorizationDecisionRef returns the default authorization decision
// as a reference.
func (c Config) DefaultAuthorizationDecisionRef() ast.Ref {
	r, _ := ref.ParseDataPath(*c.DefaultAuthorizationDecision)
	return r
}

// NDBuiltinCacheEnabled returns if the ND builtins cache should be used.
func (c Config) NDBuiltinCacheEnabled() bool {
	return c.NDBuiltinCache
}

// GetPersistenceDirectory returns the configured persistence directory, or $PWD/.opa if none is configured
func (c Config) GetPersistenceDirectory() (string, error) {
	if c.PersistenceDirectory == nil {
		pwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(pwd, ".opa"), nil
	}
	return *c.PersistenceDirectory, nil
}

// ActiveConfig returns OPA's active configuration
// with the credentials and crypto keys removed
func (c *Config) ActiveConfig() (any, error) {
	bs, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := util.UnmarshalJSON(bs, &result); err != nil {
		return nil, err
	}
	for k, e := range c.Extra {
		var v any
		if err := util.UnmarshalJSON(e, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}

	if err := removeServiceCredentials(result["services"]); err != nil {
		return nil, err
	}

	if err := removeCryptoKeys(result["keys"]); err != nil {
		return nil, err
	}

	return result, nil
}

// Clone creates a deep copy of the Config struct
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	clone := &Config{
		NDBuiltinCache: c.NDBuiltinCache,
		Server:         c.Server.Clone(),
		Storage:        c.Storage.Clone(),
		Labels:         maps.Clone(c.Labels),
	}

	if c.Services != nil {
		clone.Services = make(json.RawMessage, len(c.Services))
		copy(clone.Services, c.Services)
	}
	if c.Discovery != nil {
		clone.Discovery = make(json.RawMessage, len(c.Discovery))
		copy(clone.Discovery, c.Discovery)
	}
	if c.Bundle != nil {
		clone.Bundle = make(json.RawMessage, len(c.Bundle))
		copy(clone.Bundle, c.Bundle)
	}
	if c.Bundles != nil {
		clone.Bundles = make(json.RawMessage, len(c.Bundles))
		copy(clone.Bundles, c.Bundles)
	}
	if c.DecisionLogs != nil {
		clone.DecisionLogs = make(json.RawMessage, len(c.DecisionLogs))
		copy(clone.DecisionLogs, c.DecisionLogs)
	}
	if c.Status != nil {
		clone.Status = make(json.RawMessage, len(c.Status))
		copy(clone.Status, c.Status)
	}
	if c.Keys != nil {
		clone.Keys = make(json.RawMessage, len(c.Keys))
		copy(clone.Keys, c.Keys)
	}
	if c.Caching != nil {
		clone.Caching = make(json.RawMessage, len(c.Caching))
		copy(clone.Caching, c.Caching)
	}
	if c.DistributedTracing != nil {
		clone.DistributedTracing = make(json.RawMessage, len(c.DistributedTracing))
		copy(clone.DistributedTracing, c.DistributedTracing)
	}
	if c.MetricsExport != nil {
		clone.MetricsExport = make(json.RawMessage, len(c.MetricsExport))
		copy(clone.MetricsExport, c.MetricsExport)
	}

	if c.DefaultDecision != nil {
		s := *c.DefaultDecision
		clone.DefaultDecision = &s
	}
	if c.DefaultAuthorizationDecision != nil {
		s := *c.DefaultAuthorizationDecision
		clone.DefaultAuthorizationDecision = &s
	}
	if c.PersistenceDirectory != nil {
		s := *c.PersistenceDirectory
		clone.PersistenceDirectory = &s
	}

	if c.Plugins != nil {
		clone.Plugins = make(map[string]json.RawMessage, len(c.Plugins))
		for k, v := range c.Plugins {
			if v != nil {
				clone.Plugins[k] = make(json.RawMessage, len(v))
				copy(clone.Plugins[k], v)
			}
		}
	}

	if c.Extra != nil {
		clone.Extra = make(map[string]json.RawMessage, len(c.Extra))
		for k, v := range c.Extra {
			if v != nil {
				clone.Extra[k] = make(json.RawMessage, len(v))
				copy(clone.Extra[k], v)
			}
		}
	}

	if c.Warnings != nil {
		clone.Warnings = make([]string, len(c.Warnings))
		copy(clone.Warnings, c.Warnings)
	}

	return clone
}

func removeServiceCredentials(x any) error {
	switch x := x.(type) {
	case nil:
		return nil
	case []any:
		for _, v := range x {
			err := removeKey(v, "credentials")
			if err != nil {
				return err
			}
		}

	case map[string]any:
		for _, v := range x {
			err := removeKey(v, "credentials")
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("illegal service config type: %T", x)
	}

	return nil
}

func removeCryptoKeys(x any) error {
	switch x := x.(type) {
	case nil:
		return nil
	case map[string]any:
		for _, v := range x {
			err := removeKey(v, "key", "private_key")
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("illegal keys config type: %T", x)
	}

	return nil
}

func removeKey(x any, keys ...string) error {
	val, ok := x.(map[string]any)
	if !ok {
		return errors.New("type assertion error")
	}

	for _, key := range keys {
		delete(val, key)
	}

	return nil
}
