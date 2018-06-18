// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package logs implements decision log buffering and uploading.
package logs

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/rest"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// EventV1 represents a decision log event.
type EventV1 struct {
	Labels      map[string]string `json:"labels"`
	DecisionID  string            `json:"decision_id"`
	Revision    string            `json:"revision,omitempty"`
	Path        string            `json:"path"`
	Input       *interface{}      `json:"input,omitempty"`
	Result      *interface{}      `json:"result,omitempty"`
	RequestedBy string            `json:"requested_by"`
	Timestamp   time.Time         `json:"timestamp"`
}

const (
	// min amount of time to wait following a failure
	minRetryDelay               = time.Millisecond * 100
	defaultMinDelaySeconds      = int64(300)
	defaultMaxDelaySeconds      = int64(600)
	defaultUploadSizeLimitBytes = int64(32768) // 32KB limit
	defaultBufferSizeLimitBytes = int64(0)     // unlimited
)

// ReportingConfig represents configuration for the plugin's reporting behaviour.
type ReportingConfig struct {
	BufferSizeLimitBytes *int64 `json:"buffer_size_limit_bytes,omitempty"` // max size of in-memory buffer
	UploadSizeLimitBytes *int64 `json:"upload_size_limit_bytes,omitempty"` // max size of upload payload
	MinDelaySeconds      *int64 `json:"min_delay_seconds,omitempty"`       // min amount of time to wait between successful poll attempts
	MaxDelaySeconds      *int64 `json:"max_delay_seconds,omitempty"`       // max amount of time to wait between poll attempts
}

// Config represents the plugin configuration.
type Config struct {
	Service       string          `json:"service"`
	PartitionName string          `json:"partition_name,omitempty"`
	Reporting     ReportingConfig `json:"reporting"`
}

func (c *Config) validateAndInjectDefaults(services []string) error {

	found := false

	for _, svc := range services {
		if svc == c.Service {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("invalid service name %q in decision_logs", c.Service)
	}

	min := defaultMinDelaySeconds
	max := defaultMaxDelaySeconds

	// reject bad min/max values
	if c.Reporting.MaxDelaySeconds != nil && c.Reporting.MinDelaySeconds != nil {
		if *c.Reporting.MaxDelaySeconds < *c.Reporting.MinDelaySeconds {
			return fmt.Errorf("max reporting delay must be >= min reporting delay in decision_logs")
		}
		min = *c.Reporting.MinDelaySeconds
		max = *c.Reporting.MaxDelaySeconds
	} else if c.Reporting.MaxDelaySeconds == nil && c.Reporting.MinDelaySeconds != nil {
		return fmt.Errorf("reporting configuration missing 'max_delay_seconds' in decision_logs")
	} else if c.Reporting.MinDelaySeconds == nil && c.Reporting.MaxDelaySeconds != nil {
		return fmt.Errorf("reporting configuration missing 'min_delay_seconds' in decision_logs")
	}

	// scale to seconds
	minSeconds := int64(time.Duration(min) * time.Second)
	c.Reporting.MinDelaySeconds = &minSeconds

	maxSeconds := int64(time.Duration(max) * time.Second)
	c.Reporting.MaxDelaySeconds = &maxSeconds

	// default the upload size limit
	uploadLimit := defaultUploadSizeLimitBytes
	if c.Reporting.UploadSizeLimitBytes != nil {
		uploadLimit = *c.Reporting.UploadSizeLimitBytes
	}

	c.Reporting.UploadSizeLimitBytes = &uploadLimit

	// default the buffer size limit
	bufferLimit := defaultBufferSizeLimitBytes
	if c.Reporting.BufferSizeLimitBytes != nil {
		bufferLimit = *c.Reporting.BufferSizeLimitBytes
	}

	c.Reporting.BufferSizeLimitBytes = &bufferLimit

	return nil
}

// Plugin implements decision log buffering and uploading.
type Plugin struct {
	manager *plugins.Manager
	config  Config
	buffer  *logBuffer
	enc     *chunkEncoder
	mtx     sync.Mutex
	stop    chan chan struct{}
}

// New returns a new Plugin with the given config.
func New(config []byte, manager *plugins.Manager) (*Plugin, error) {

	var parsedConfig Config

	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return nil, err
	}

	if err := parsedConfig.validateAndInjectDefaults(manager.Services()); err != nil {
		return nil, err
	}

	plugin := &Plugin{
		manager: manager,
		config:  parsedConfig,
		stop:    make(chan chan struct{}),
		buffer:  newLogBuffer(*parsedConfig.Reporting.BufferSizeLimitBytes),
		enc:     newChunkEncoder(*parsedConfig.Reporting.UploadSizeLimitBytes),
	}

	return plugin, nil
}

// Start starts the plugin.
func (p *Plugin) Start(ctx context.Context) error {
	go p.loop()
	return nil
}

// Stop stops the plugin.
func (p *Plugin) Stop(ctx context.Context) {
	done := make(chan struct{})
	p.stop <- done
	_ = <-done
}

// Log appends a decision log event to the buffer for uploading.
func (p *Plugin) Log(ctx context.Context, decision *server.Info) {
	path := strings.Replace(strings.TrimPrefix(decision.Query, "data."), ".", "/", -1)

	event := EventV1{
		Labels:      p.manager.Labels,
		DecisionID:  decision.DecisionID,
		Revision:    decision.Revision,
		Path:        path,
		Input:       &decision.Input,
		Result:      decision.Results,
		RequestedBy: decision.RemoteAddr,
		Timestamp:   decision.Timestamp,
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()

	result, err := p.enc.Write(event)
	if err != nil {
		p.logError("Log encoding failed: %v.", err)
		return
	}

	if result != nil {
		p.bufferChunk(p.buffer, result)
	}
}

func (p *Plugin) loop() {

	ctx, cancel := context.WithCancel(context.Background())

	var retry int

	for {
		uploaded, err := p.oneShot(ctx)

		if err != nil {
			p.logError("%v.", err)
		} else if uploaded {
			p.logInfo("Logs uploaded successfully.")
		} else {
			p.logInfo("Log upload skipped.")
		}

		var delay time.Duration

		if err == nil {
			min := float64(*p.config.Reporting.MinDelaySeconds)
			max := float64(*p.config.Reporting.MaxDelaySeconds)
			delay = time.Duration(((max - min) * rand.Float64()) + min)
		} else {
			delay = util.DefaultBackoff(float64(minRetryDelay), float64(*p.config.Reporting.MaxDelaySeconds), retry)
		}

		p.logDebug("Waiting %v before next upload/retry.", delay)
		timer := time.NewTimer(delay)

		select {
		case <-timer.C:
			if err != nil {
				retry++
			} else {
				retry = 0
			}
		case done := <-p.stop:
			cancel()
			done <- struct{}{}
			return
		}
	}
}

func (p *Plugin) oneShot(ctx context.Context) (ok bool, err error) {
	// Make a local copy of the plugins's encoder and buffer and create
	// a new encoder and buffer. This is needed as locking the buffer for
	// the upload duration will block policy evaluation and result in
	// increased latency for OPA clients
	p.mtx.Lock()
	oldChunkEnc := p.enc
	oldBuffer := p.buffer
	p.buffer = newLogBuffer(*p.config.Reporting.BufferSizeLimitBytes)
	p.enc = newChunkEncoder(*p.config.Reporting.UploadSizeLimitBytes)
	p.mtx.Unlock()

	// Along with uploading the compressed events in the buffer
	// to the remote server, flush any pending compressed data to the
	// underlying writer and add to the buffer.
	chunk, err := oldChunkEnc.Flush()
	if err != nil {
		return false, err
	} else if chunk != nil {
		p.bufferChunk(oldBuffer, chunk)
	}

	if oldBuffer.Len() == 0 {
		return false, nil
	}

	for bs := oldBuffer.Pop(); bs != nil; bs = oldBuffer.Pop() {
		err := uploadChunk(ctx, p.manager.Client(p.config.Service), p.config.PartitionName, bs)
		if err != nil {
			// requeue the chunk
			p.mtx.Lock()
			p.bufferChunk(p.buffer, bs)
			p.mtx.Unlock()
			return false, err
		}
	}

	return true, nil
}

func (p *Plugin) bufferChunk(buffer *logBuffer, bs []byte) {
	dropped := buffer.Push(bs)
	if dropped > 0 {
		p.logError("Dropped %v chunks from buffer. Reduce reporting interval or increase buffer size.", dropped)
	}
}

func uploadChunk(ctx context.Context, client rest.Client, partitionName string, data []byte) error {

	resp, err := client.
		WithHeader("Content-Type", "application/json").
		WithHeader("Content-Encoding", "gzip").
		WithBytes(data).
		Do(ctx, "POST", fmt.Sprintf("/logs/%v", partitionName))

	if err != nil {
		return errors.Wrap(err, "Log upload failed")
	}

	defer util.Close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("Log upload failed, server replied with not found")
	case http.StatusUnauthorized:
		return fmt.Errorf("Log upload failed, server replied with not authorized")
	default:
		return fmt.Errorf("Log upload failed, server replied with HTTP %v", resp.StatusCode)
	}
}

func (p *Plugin) logError(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Errorf(fmt, a...)
}

func (p *Plugin) logInfo(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Infof(fmt, a...)
}

func (p *Plugin) logDebug(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Debugf(fmt, a...)
}

func (p *Plugin) logrusFields() logrus.Fields {
	return logrus.Fields{
		"plugin": "decision_logs",
	}
}
