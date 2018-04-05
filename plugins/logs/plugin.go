// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package logs implements decision log buffering and uploading.
package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/plugins"
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
	defaultUploadSizeLimitBytes = int64(32768)   // 32KB limit
	defaultBufferSizeLimitBytes = int64(1048576) // 1MB limit
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

	var buf bytes.Buffer

	path := strings.Replace(strings.TrimLeft(decision.Query, "data."), ".", "/", -1)

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

	if err := json.NewEncoder(&buf).Encode(event); err != nil {
		p.logError("Log serialization failed: %v.", err)
		return
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()
	dropped := p.buffer.Push(buf.Bytes(), false)
	if dropped > 0 {
		p.logInfo("Dropped %v events from buffer. Reduce reporting interval or increase buffer size.")
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

	chunks, err := p.chunkBuffer()
	if err != nil {
		return false, errors.Wrap(err, "Log processing failed")
	}

	var chunkIndex int

	defer func() {
		if err != nil {
			p.requeueChunks(chunks, chunkIndex)
		}
	}()

	for chunkIndex = range chunks {

		resp, err := p.manager.Client(p.config.Service).
			WithHeader("Content-Type", "application/json").
			WithHeader("Content-Encoding", "gzip").
			WithBytes(chunks[chunkIndex]).
			Do(ctx, "POST", fmt.Sprintf("/logs/%v", p.config.PartitionName))

		if err != nil {
			return false, errors.Wrap(err, "Log upload failed")
		}

		defer util.Close(resp)

		switch resp.StatusCode {
		case http.StatusOK:
			break
		case http.StatusNotFound:
			return false, fmt.Errorf("Log upload failed, server replied with not found")
		case http.StatusUnauthorized:
			return false, fmt.Errorf("Log upload failed, server replied with not authorized")
		default:
			return false, fmt.Errorf("Log upload failed, server replied with HTTP %v", resp.StatusCode)
		}
	}

	return len(chunks) > 0, nil
}

func (p *Plugin) chunkBuffer() ([][]byte, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return chunk(*p.config.Reporting.UploadSizeLimitBytes, p.buffer)
}

func (p *Plugin) requeueChunks(chunks [][]byte, idx int) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var dropped int

	for ; idx < len(chunks); idx++ {
		dropped += p.buffer.Push(chunks[idx], true)
	}

	if dropped > 0 {
		p.logInfo("Dropped %v events from buffer. Reduce reporting interval or increase buffer size.")
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

func chunk(limit int64, buffer *logBuffer) ([][]byte, error) {

	enc := newChunkEncoder(limit)
	chunks := [][]byte{}

	for bs, isChunk := buffer.Pop(); bs != nil; bs, isChunk = buffer.Pop() {
		if !isChunk {
			chunk, err := enc.Write(bs)
			if err != nil {
				return nil, err
			} else if chunk != nil {
				chunks = append(chunks, chunk)
			}
		} else {
			chunks = append(chunks, bs)
		}
	}

	chunk, err := enc.Flush()
	if err != nil {
		return nil, err
	} else if chunk != nil {
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}
