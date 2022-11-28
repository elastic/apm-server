// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package beatcmd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
)

// NewRunnerFunc is a function type that constructs a new Runner with the given
// parameters.
type NewRunnerFunc func(RunnerParams) (Runner, error)

// RunnerParams holds parameters that will be passed by Reloader
// to its NewRunnerFunc.
type RunnerParams struct {
	// Config holds the full, raw, configuration, including apm-server.*
	// and output.* attributes.
	Config *config.C

	// Info holds information about the APM Server ("beat", for historical
	// reasons) process.
	Info beat.Info

	// Logger holds a logger to use for logging throughout the APM Server.
	Logger *logp.Logger
}

// Runner is an interface returned by NewRunnerFunc.
type Runner interface {
	// Run runs until its context is cancelled.
	Run(context.Context) error
}

// NewReloader returns a new Reloader which creates Runners using the provided
// beat.Info and NewRunnerFunc.
func NewReloader(info beat.Info, newRunner NewRunnerFunc) (*Reloader, error) {
	r := &Reloader{
		info:      info,
		logger:    logp.NewLogger(""),
		newRunner: newRunner,
		stopped:   make(chan struct{}),
	}
	if err := reload.RegisterV2.RegisterList(reload.InputRegName, reloadableListFunc(r.reloadInputs)); err != nil {
		return nil, fmt.Errorf("failed to register inputs reloader: %w", err)
	}
	if err := reload.RegisterV2.Register(reload.OutputRegName, reload.ReloadableFunc(r.reloadOutput)); err != nil {
		return nil, fmt.Errorf("failed to register output reloader: %w", err)
	}
	return r, nil
}

// Reloader responds to libbeat configuration changes by calling the given
// NewRunnerFunc to create a new Runner, and then stopping any existing one.
type Reloader struct {
	info      beat.Info
	logger    *logp.Logger
	newRunner NewRunnerFunc

	runner     Runner
	stopRunner func() error

	mu            sync.Mutex
	inputRevision int64
	inputConfig   *config.C
	outputConfig  *config.C
	stopped       chan struct{}
}

// Run runs the Reloader, blocking until ctx is cancelled or a fatal error occurs.
//
// Run must be called once and only once.
func (r *Reloader) Run(ctx context.Context) error {
	defer close(r.stopped)
	<-ctx.Done()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runner == nil {
		return nil
	}
	return r.stopRunner()
}

// reloadInput (re)loads input configuration.
//
// Note: reloadInputs may be called before the Reloader is running.
func (r *Reloader) reloadInputs(configs []*reload.ConfigWithMeta) error {
	if n := len(configs); n != 1 {
		return fmt.Errorf("only 1 input supported, got %d", n)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg := configs[0].Config

	// Input configuration is expected to have a monotonically
	// increasing revision number.
	//
	// Suppress config input changes that are no newer than
	// the most recently loaded configuration.
	revision, err := cfg.Int("revision", -1)
	if err != nil {
		return fmt.Errorf("failed to extract input config revision: %w", err)
	}
	if r.inputConfig != nil && revision <= r.inputRevision {
		r.logger.With(
			logp.Int64("revision", revision),
		).Debug("suppressing stale input config change")
		return nil
	}

	if err := r.reload(cfg, r.outputConfig); err != nil {
		return fmt.Errorf("failed to load input config: %w", err)
	}
	r.inputRevision = revision
	r.inputConfig = cfg
	r.logger.With(logp.Int64("revision", revision)).Info("loaded input config")
	return nil
}

// reloadOutput (re)loads output configuration.
//
// Note: reloadOutput may be called before the Reloader is running.
func (r *Reloader) reloadOutput(cfg *reload.ConfigWithMeta) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.outputConfig != nil && configEqual(cfg.Config, r.outputConfig) {
		r.logger.Debug("suppressing redundant output config change")
		return nil
	}
	if err := r.reload(r.inputConfig, cfg.Config); err != nil {
		return fmt.Errorf("failed to load output config: %w", err)
	}
	r.outputConfig = cfg.Config
	r.logger.Info("loaded output config")
	return nil
}

func (r *Reloader) reload(inputConfig, outputConfig *config.C) error {
	var outputNamespace config.Namespace
	if outputConfig != nil {
		if err := outputConfig.Unpack(&outputNamespace); err != nil {
			return err
		}
	}
	if inputConfig == nil || !outputNamespace.IsSet() {
		// Wait until both input and output have been received.
		return nil
	}
	select {
	case <-r.stopped:
		// The process is shutting down: ignore reloads.
		return nil
	default:
	}

	wrappedOutputConfig := config.MustNewConfigFrom(map[string]interface{}{
		"output": outputConfig,
	})
	mergedConfig, err := config.MergeConfigs(inputConfig, wrappedOutputConfig)
	if err != nil {
		return err
	}

	// Create a new runner. We separate creation from starting to
	// allow the runner to perform initialisations that must run
	// synchronously.
	newRunner, err := r.newRunner(RunnerParams{
		Config: mergedConfig,
		Info:   r.info,
		Logger: r.logger,
	})
	if err != nil {
		return err
	}

	// Start the new runner.
	var g errgroup.Group
	ctx, cancel := context.WithCancel(context.Background())
	g.Go(func() error {
		if err := newRunner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.logger.With(logp.Error(err)).Error("runner returned with an error")
			return err
		}
		return nil
	})
	stopRunner := func() error {
		cancel()
		return g.Wait()
	}

	// Stop any existing runner.
	if r.runner != nil {
		_ = r.stopRunner() // logged above
	}
	r.runner = newRunner
	r.stopRunner = stopRunner

	return nil
}

type reloadableListFunc func(config []*reload.ConfigWithMeta) error

func (f reloadableListFunc) Reload(configs []*reload.ConfigWithMeta) error {
	return f(configs)
}

// configEqual tells us whether the two config structures are equal, by
// unpacking them into map[string]interface{} and using reflect.DeepEqual.
func configEqual(a, b *config.C) bool {
	var ma, mb map[string]interface{}
	_ = a.Unpack(&ma)
	_ = b.Unpack(&mb)
	return reflect.DeepEqual(ma, mb)
}
