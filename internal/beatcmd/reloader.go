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
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/joeshaw/multierror"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/cfgfile"
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
func NewReloader(info beat.Info, registry *reload.Registry, newRunner NewRunnerFunc) (*Reloader, error) {
	r := &Reloader{
		info:      info,
		logger:    logp.NewLogger(""),
		newRunner: newRunner,
		stopped:   make(chan struct{}),
	}
	if err := registry.RegisterList(reload.InputRegName, reloadableListFunc(r.reloadInputs)); err != nil {
		return nil, fmt.Errorf("failed to register inputs reloader: %w", err)
	}
	if err := registry.Register(reload.OutputRegName, reload.ReloadableFunc(r.reloadOutput)); err != nil {
		return nil, fmt.Errorf("failed to register output reloader: %w", err)
	}
	if err := registry.Register(reload.APMRegName, reload.ReloadableFunc(r.reloadAPMTracing)); err != nil {
		return nil, fmt.Errorf("failed to register apm tracing reloader: %w", err)
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

	mu               sync.Mutex
	inputConfig      *config.C
	outputConfig     *config.C
	apmTracingConfig *config.C
	stopped          chan struct{}
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
// It returns a *multierror.MultiError as libbeat manager error handling is tightly coupled
// with its own reloadable list implementation in libbeat/cfgfile/list.go.
//
// Note: reloadInputs may be called before the Reloader is running.
func (r *Reloader) reloadInputs(configs []*reload.ConfigWithMeta) error {
	if n := len(configs); n != 1 {
		var errs multierror.Errors
		for _, cfg := range configs {
			unitErr := cfgfile.UnitError{
				Err:    fmt.Errorf("only 1 input supported, got %d", n),
				UnitID: cfg.InputUnitID,
			}
			errs = append(errs, unitErr)
		}
		return errs.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg := configs[0].Config

	// Input configuration is expected to have a monotonically
	// increasing revision number.
	revision, err := cfg.Int("revision", -1)
	if err != nil {
		return multierror.Errors{
			cfgfile.UnitError{
				Err:    fmt.Errorf("failed to extract input config revision: %w", err),
				UnitID: configs[0].InputUnitID,
			},
		}.Err()
	}

	if err := r.reload(cfg, r.outputConfig, r.apmTracingConfig); err != nil {
		return multierror.Errors{
			cfgfile.UnitError{
				Err:    fmt.Errorf("failed to load input config: %w", err),
				UnitID: configs[0].InputUnitID,
			},
		}.Err()
	}
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
	if err := r.reload(r.inputConfig, cfg.Config, r.apmTracingConfig); err != nil {
		return fmt.Errorf("failed to load output config: %w", err)
	}
	r.outputConfig = cfg.Config
	r.logger.Info("loaded output config")
	return nil
}

// reloadAPMTracing (re)loads apm tracing configuration.
func (r *Reloader) reloadAPMTracing(cfg *reload.ConfigWithMeta) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var c *config.C
	if cfg != nil {
		c = cfg.Config
	}
	if err := r.reload(r.inputConfig, r.outputConfig, c); err != nil {
		return fmt.Errorf("failed to load apm tracing config: %w", err)
	}
	r.apmTracingConfig = c
	r.logger.Info("loaded apm tracing config")
	return nil
}

func (r *Reloader) reload(inputConfig, outputConfig, apmTracingConfig *config.C) error {
	var outputNamespace config.Namespace
	if outputConfig != nil {
		if err := outputConfig.Unpack(&outputNamespace); err != nil {
			return err
		}
	}
	if inputConfig == nil || !outputNamespace.IsSet() {
		// Wait until both input and output have been received.
		// apm tracing config is not mandatory so not waiting for it
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

	var wrappedApmTracingConfig *config.C
	// apmTracingConfig is nil when disabled
	if apmTracingConfig != nil {
		c, err := apmTracingConfig.Child("elastic", -1)
		if err != nil {
			return fmt.Errorf("APM tracing config for elastic not found")
		}
		// set enabled manually as APMConfig doesn't contain it.
		c.SetBool("enabled", -1, true)
		wrappedApmTracingConfig = config.MustNewConfigFrom(map[string]interface{}{
			"instrumentation": c,
		})
	} else {
		// empty instrumentation config
		wrappedApmTracingConfig = config.NewConfig()
	}
	mergedConfig, err := config.MergeConfigs(inputConfig, wrappedOutputConfig, wrappedApmTracingConfig)
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
