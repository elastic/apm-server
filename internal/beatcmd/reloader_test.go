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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/elastic-agent-libs/config"
)

func TestReloader(t *testing.T) {
	oldRegistry := reload.RegisterV2
	defer func() { reload.RegisterV2 = oldRegistry }()
	reload.RegisterV2 = reload.NewRegistry()

	type runner struct {
		running chan struct{}
		stopped chan struct{}
	}
	runners := make(chan runner, 1)
	assertNoReload := func() {
		select {
		case <-runners:
			t.Fatal("unexpected runner")
		case <-time.After(50 * time.Millisecond):
		}
	}
	assertReload := func() runner {
		select {
		case r := <-runners:
			return r
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for runner to be created")
		}
		panic("unreachable")
	}

	reloader, err := NewReloader(beat.Info{}, func(args RunnerParams) (Runner, error) {
		if shouldError, _ := args.Config.Bool("error", -1); shouldError {
			return nil, errors.New("no runner for you")
		}
		runner := runner{
			running: make(chan struct{}),
			stopped: make(chan struct{}),
		}
		runners <- runner
		return runnerFunc(func(ctx context.Context) error {
			close(runner.running)
			defer close(runner.stopped)
			<-ctx.Done()
			return nil
		}), nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return reloader.Run(ctx) })
	defer func() { assert.NoError(t, g.Wait()) }()
	defer cancel()

	// No reload until there's input and output configuration.
	assertNoReload()

	err = reload.RegisterV2.GetInputList().Reload([]*reload.ConfigWithMeta{{
		Config: config.MustNewConfigFrom(`{}`),
	}})
	assert.EqualError(t, err, "failed to extract input config revision: missing field accessing 'revision'")
	assertNoReload()

	err = reload.RegisterV2.GetInputList().Reload([]*reload.ConfigWithMeta{{
		Config: config.MustNewConfigFrom(`{"revision": 1}`),
	}})
	assert.NoError(t, err)
	assertNoReload()

	err = reload.RegisterV2.GetReloadableOutput().Reload(&reload.ConfigWithMeta{
		Config: config.MustNewConfigFrom(`{}`),
	})
	assert.NoError(t, err)
	assertNoReload() // an output must be set

	err = reload.RegisterV2.GetReloadableOutput().Reload(&reload.ConfigWithMeta{
		Config: config.MustNewConfigFrom(`{"console.enabled": true}`),
	})
	assert.NoError(t, err)
	r1 := assertReload() // both input and output configuration are defined
	assertNoReload()

	expectEvent(t, r1.running, "runner should have been started")
	expectNoEvent(t, r1.stopped, "runner should not have been stopped")

	err = reload.RegisterV2.GetInputList().Reload([]*reload.ConfigWithMeta{{
		Config: config.MustNewConfigFrom(`{"revision": 1}`),
	}})
	assert.NoError(t, err)
	assertNoReload() // reload suppressed, revision is the same

	err = reload.RegisterV2.GetReloadableOutput().Reload(&reload.ConfigWithMeta{
		Config: config.MustNewConfigFrom(`{"console.enabled": true}`),
	})
	assert.NoError(t, err)
	assertNoReload() // reload suppressed, config is the same

	err = reload.RegisterV2.GetInputList().Reload([]*reload.ConfigWithMeta{{
		Config: config.MustNewConfigFrom(`{"revision": 2, "error": true}`),
	}})
	assert.EqualError(t, err, "failed to load input config: no runner for you")
	assertNoReload() // error occurred during reload, nothing changes
	expectNoEvent(t, r1.stopped, "runner should not have been stopped")

	err = reload.RegisterV2.GetInputList().Reload([]*reload.ConfigWithMeta{{
		Config: config.MustNewConfigFrom(`{"revision": 3}`),
	}})
	assert.NoError(t, err)
	r2 := assertReload()
	expectEvent(t, r1.stopped, "old runner should have been stopped")
	expectEvent(t, r2.running, "new runner should have been started")
	expectNoEvent(t, r2.stopped, "new runner should not have been stopped")

	cancel()
	expectEvent(t, r2.stopped, "runner should have been stopped")
}

func TestReloaderNewRunnerParams(t *testing.T) {
	oldRegistry := reload.RegisterV2
	defer func() { reload.RegisterV2 = oldRegistry }()
	reload.RegisterV2 = reload.NewRegistry()

	calls := make(chan RunnerParams, 1)
	info := beat.Info{Beat: "not-apm-server", Version: "0.0.1"}
	reloader, err := NewReloader(info, func(args RunnerParams) (Runner, error) {
		calls <- args
		return runnerFunc(func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}), nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return reloader.Run(ctx) })
	defer func() { assert.NoError(t, g.Wait()) }()
	defer cancel()

	reload.RegisterV2.GetInputList().Reload([]*reload.ConfigWithMeta{{
		Config: config.MustNewConfigFrom(`{"revision": 1, "input": 123}`),
	}})
	reload.RegisterV2.GetReloadableOutput().Reload(&reload.ConfigWithMeta{
		Config: config.MustNewConfigFrom(`{"console.enabled": true}`),
	})
	args := <-calls
	assert.NotNil(t, args.Logger)
	assert.Equal(t, info, args.Info)
	assert.Equal(t, config.MustNewConfigFrom(`{"revision": 1, "input": 123, "output.console.enabled": true}`), args.Config)
}

func expectNoEvent(t testing.TB, ch <-chan struct{}, message string) {
	select {
	case <-ch:
		t.Fatalf("unexpected event: %s", message)
	case <-time.After(50 * time.Millisecond):
	}
}

func expectEvent(t testing.TB, ch <-chan struct{}, message string) {
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for event: %s", message)
	}
}
