// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
)

type Tester struct {
	startErr error
}

func (t *Tester) Start(_ context.Context) error {
	return t.startErr
}

func (t *Tester) Stop(_ context.Context) {}

func (t *Tester) Reconfigure(_ context.Context, _ interface{}) {}

type Config struct {
	ConfigErr bool `json:"configerr"`
}

type Factory struct{}

func (f Factory) Validate(_ *plugins.Manager, config []byte) (interface{}, error) {

	cfg := Config{}

	if err := util.Unmarshal(config, &cfg); err != nil {
		return nil, err
	}

	if cfg.ConfigErr {
		return nil, errors.New("test error")
	}

	return cfg, nil
}

func (f Factory) New(_ *plugins.Manager, _ interface{}) plugins.Plugin {
	return &Tester{}
}

func TestRegisterPlugin(t *testing.T) {

	params := NewParams()

	fs := map[string]string{
		"/config.yaml": `{"plugins": {"test": {}}}`,
	}

	test.WithTempFS(fs, func(testDirRoot string) {

		RegisterPlugin("test", Factory{})

		params.ConfigFile = filepath.Join(testDirRoot, "/config.yaml")

		rt, err := NewRuntime(context.Background(), params)
		if err != nil {
			t.Fatal(err.Error())
		}

		if err := rt.Manager.Start(context.Background()); err != nil {
			t.Fatalf("Unable to initialize plugins: %v", err.Error())
		}

		p := rt.Manager.Plugin("test")
		if p == nil {
			t.Fatal("expected plugin to be registered")
		}

	})

}

func TestRegisterPluginNotStartedWithoutConfig(t *testing.T) {

	params := NewParams()

	fs := map[string]string{
		"/config.yaml": `{"plugins": {}}`,
	}

	test.WithTempFS(fs, func(testDirRoot string) {

		RegisterPlugin("test", Factory{})

		params.ConfigFile = filepath.Join(testDirRoot, "/config.yaml")

		rt, err := NewRuntime(context.Background(), params)
		if err != nil {
			t.Fatal(err.Error())
		}

		if err := rt.Manager.Start(context.Background()); err != nil {
			t.Fatalf("Unable to initialize plugins: %v", err.Error())
		}

		p := rt.Manager.Plugin("test")
		if p != nil {
			t.Fatal("expected plugin to be missing")
		}

	})

}

func TestRegisterPluginBadBootConfig(t *testing.T) {

	params := NewParams()

	fs := map[string]string{
		"/config.yaml": `{"plugins": {"test": {"configerr": true}}}`,
	}

	test.WithTempFS(fs, func(testDirRoot string) {

		RegisterPlugin("test", Factory{})

		params.ConfigFile = filepath.Join(testDirRoot, "/config.yaml")

		_, err := NewRuntime(context.Background(), params)
		if err == nil || !strings.Contains(err.Error(), "config error: test") {
			t.Fatal("expected config error but got:", err)
		}

	})

}

func TestWaitPluginsReady(t *testing.T) {
	fs := map[string]string{
		"/config.yaml": `{"plugins": {"test": {}}}`,
	}

	test.WithTempFS(fs, func(testDirRoot string) {

		RegisterPlugin("test", Factory{})

		params := NewParams()
		params.ConfigFile = filepath.Join(testDirRoot, "/config.yaml")

		rt, err := NewRuntime(context.Background(), params)
		if err != nil {
			t.Fatal(err.Error())
		}

		if err := rt.Manager.Start(context.Background()); err != nil {
			t.Fatalf("Unable to initialize plugins: %v", err.Error())
		}

		rt.Manager.UpdatePluginStatus("test", &plugins.Status{
			State: plugins.StateNotReady,
		})

		go func() {
			time.Sleep(100 * time.Millisecond)

			rt.Manager.UpdatePluginStatus("test", &plugins.Status{
				State: plugins.StateOK,
			})
			rt.Manager.UpdatePluginStatus("discovery", &plugins.Status{
				State: plugins.StateOK,
			})
		}()

		if err := rt.waitPluginsReady(1*time.Millisecond, 0); err != nil {
			t.Fatalf("Expected no error when no timeout: %v", err)
		}

		if err := rt.waitPluginsReady(1*time.Millisecond, 1*time.Millisecond); err == nil {
			t.Fatal("Expected timeout error")
		}

		if err := rt.waitPluginsReady(1*time.Millisecond, time.Second); err != nil {
			t.Fatal(err)
		}
	})
}
