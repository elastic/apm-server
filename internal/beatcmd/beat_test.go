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
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/internal/version"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/management"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/paths"
)

// TestRunMaxProcs ensures Beat.Run calls the GOMAXPROCS adjustment code by looking for log messages.
func TestRunMaxProcs(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("%d_GOMAXPROCS", n), func(t *testing.T) {
			t.Setenv("GOMAXPROCS", strconv.Itoa(n))
			beat := newNopBeat(t, "output.console.enabled: true")
			logs := logp.ObserverLogs()

			stop := runBeat(t, beat)
			timeout := time.NewTimer(10 * time.Second)
			defer timeout.Stop()
			for {
				select {
				case <-timeout.C:
					t.Error("timed out waiting for log message, total logs observed:", logs.Len())
					for _, log := range logs.All() {
						t.Log(log.LoggerName, log.Message)
					}
					return
				case <-time.After(10 * time.Millisecond):
				}

				logs := logs.FilterMessageSnippet(fmt.Sprintf(
					`maxprocs: Honoring GOMAXPROCS="%d" as set in environment`, n,
				))
				if logs.Len() > 0 {
					break
				}
			}

			assert.NoError(t, stop())
		})
	}
}

func TestRunnerParams(t *testing.T) {
	calls := make(chan RunnerParams, 1)
	b := newBeat(t, "output.console.enabled: true", func(args RunnerParams) (Runner, error) {
		calls <- args
		return newNopRunner(args), nil
	})
	stop := runBeat(t, b)
	args := expectRunnerParams(t, calls)
	assert.NoError(t, stop())

	assert.Equal(t, "apm-server", args.Info.Beat)
	assert.Equal(t, version.Version, args.Info.Version)
	assert.True(t, args.Info.ElasticLicensed)
	assert.NotZero(t, args.Info.ID)
	assert.NotZero(t, args.Info.EphemeralID)
	assert.NotZero(t, args.Info.FirstStart)
	assert.NotZero(t, args.Info.StartTime)
	hostname, _ := os.Hostname()
	assert.Equal(t, hostname, args.Info.Hostname)

	assert.NotNil(t, args.Logger)

	var m map[string]interface{}
	require.NotNil(t, args.Config)
	err := args.Config.Unpack(&m)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"output": map[string]interface{}{
			"console": map[string]interface{}{
				"enabled": true,
			},
		},
		"path": map[string]interface{}{
			"config": paths.Paths.Config,
			"logs":   paths.Paths.Logs,
			"data":   paths.Paths.Data,
			"home":   paths.Paths.Home,
		},
	}, m)
}

func TestUnmanagedOutputRequired(t *testing.T) {
	b := newBeat(t, "", func(args RunnerParams) (Runner, error) {
		panic("unreachable")
	})
	err := b.Run(context.Background())
	assert.EqualError(t, err, "no output defined, please define one under the output section")
}

func TestRunManager(t *testing.T) {
	oldRegistry := reload.RegisterV2
	defer func() { reload.RegisterV2 = oldRegistry }()
	reload.RegisterV2 = reload.NewRegistry()

	// Register our own mock management implementation.
	manager := newMockManager()
	management.SetManagerFactory(func(cfg *config.C, registry *reload.Registry) (management.Manager, error) {
		return manager, nil
	})

	calls := make(chan RunnerParams, 1)
	b := newBeat(t, "management.enabled: true", func(args RunnerParams) (Runner, error) {
		calls <- args
		return newNopRunner(args), nil
	})

	var g errgroup.Group
	g.Go(func() error { return b.Run(context.Background()) })
	expectNoRunnerParams(t, calls)

	// Mimic Elastic Agent management by waiting for the manager to be started,
	// then reloading configuration, and performing a remote shutdown by calling
	// the stop callback registered with the manager.
	expectEvent(t, manager.started, "manager should have been started")
	expectNoEvent(t, manager.stopped, "manager should not have been stopped")

	err := reload.RegisterV2.GetInputList().Reload([]*reload.ConfigWithMeta{{
		Config: config.MustNewConfigFrom(`{
			"revision": 1,
			"apm-server.host": "localhost:1234"
		}`),
	}})
	assert.NoError(t, err)
	err = reload.RegisterV2.GetReloadableOutput().Reload(&reload.ConfigWithMeta{
		Config: config.MustNewConfigFrom(`{"console.enabled": true}`),
	})
	assert.NoError(t, err)

	args := expectRunnerParams(t, calls)
	var m map[string]interface{}
	err = args.Config.Unpack(&m)
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"revision": uint64(1),
		"apm-server": map[string]interface{}{
			"host": "localhost:1234",
		},
		"output": map[string]interface{}{
			"console": map[string]interface{}{
				"enabled": true,
			},
		},
	}, m)

	require.NotNil(t, manager.stopCallback)
	manager.stopCallback()
	assert.NoError(t, g.Wait())
	expectEvent(t, manager.stopped, "manager should have been stopped")
}

func runBeat(t testing.TB, beat *Beat) (stop func() error) {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	t.Cleanup(func() { assert.NoError(t, g.Wait()) })
	t.Cleanup(cancel)
	g.Go(func() error { return beat.Run(ctx) })
	return func() error {
		cancel()
		return g.Wait()
	}
}

func newNopBeat(t testing.TB, configYAML string) *Beat {
	return newBeat(t, configYAML, func(RunnerParams) (Runner, error) {
		return runnerFunc(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}), nil
	})
}

func newBeat(t testing.TB, configYAML string, newRunner NewRunnerFunc) *Beat {
	resetGlobals()
	initCfgfile(t, configYAML)
	beat, err := NewBeat(BeatParams{
		NewRunner:       newRunner,
		ElasticLicensed: true,
	})
	require.NoError(t, err)
	return beat
}

func resetGlobals() {
	// Clear monitoring registries to allow the new Beat to populate them.
	monitoring.GetNamespace("info").SetRegistry(nil)
	monitoring.GetNamespace("state").SetRegistry(nil)
	for _, name := range []string{"system", "beat", "libbeat"} {
		registry := monitoring.Default.GetRegistry(name)
		if registry != nil {
			registry.Clear()
		}
	}

	// Create a new reload registry, as the Beat.Run method will register with it.
	reload.RegisterV2 = reload.NewRegistry()
}

type runnerFunc func(ctx context.Context) error

func (f runnerFunc) Run(ctx context.Context) error {
	return f(ctx)
}

func newNopRunner(RunnerParams) Runner {
	return runnerFunc(func(ctx context.Context) error {
		return nil
	})
}

func expectRunnerParams(t testing.TB, ch <-chan RunnerParams) RunnerParams {
	select {
	case args := <-ch:
		return args
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for NewRunnerFunc call")
	}
	panic("unreachable")
}

func expectNoRunnerParams(t testing.TB, ch <-chan RunnerParams) {
	select {
	case <-ch:
		t.Fatal("unexpected NewRunnerFunc call")
	case <-time.After(50 * time.Millisecond):
	}
}

type mockManager struct {
	management.Manager // embed nil value to panic on unimplemented methods
	enabled            bool
	started            chan struct{}
	stopped            chan struct{}
	stopCallback       func()
}

func newMockManager() *mockManager {
	return &mockManager{
		enabled: true,
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (m *mockManager) Enabled() bool {
	return m.enabled
}

func (m *mockManager) Start() error {
	close(m.started)
	return nil
}

func (m *mockManager) Stop() {
	close(m.stopped)
}

func (m *mockManager) SetStopCallback(f func()) {
	m.stopCallback = f
}
