// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

type Tester struct {
	startErr error
}

func (t *Tester) Start(ctx context.Context) error {
	return t.startErr
}

func (t *Tester) Stop(ctx context.Context) {
	return
}

func (t *Tester) Reconfigure(ctx context.Context, config interface{}) {
	return
}

type Config struct {
	ConfigErr bool `json:"configerr"`
}

type Factory struct{}

func (f Factory) Validate(_ *plugins.Manager, config []byte) (interface{}, error) {

	test := Config{}

	if err := util.Unmarshal(config, &test); err != nil {
		return nil, err
	}

	if test.ConfigErr {
		return nil, fmt.Errorf("test error")
	}

	return test, nil
}

func (f Factory) New(_ *plugins.Manager, config interface{}) plugins.Plugin {
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
			t.Fatalf(err.Error())
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
			t.Fatalf(err.Error())
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
			t.Fatalf(err.Error())
		}

		if err := rt.Manager.Start(context.Background()); err != nil {
			t.Fatalf("Unable to initialize plugins: %v", err.Error())
		}

		rt.Manager.UpdatePluginStatus("test", &plugins.Status{
			State: plugins.StateNotReady,
		})

		go func() {
			time.Sleep(2 * time.Millisecond)
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

		if err := rt.waitPluginsReady(1*time.Millisecond, 3*time.Millisecond); err != nil {
			t.Fatal(err)
		}
	})
}
