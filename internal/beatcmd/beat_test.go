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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/elastic/apm-server/internal/version"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/management"
	"github.com/elastic/beats/v7/libbeat/tests/integration"
	xpacklbmanagement "github.com/elastic/beats/v7/x-pack/libbeat/management"
	"github.com/elastic/elastic-agent-client/v7/pkg/client"
	"github.com/elastic/elastic-agent-client/v7/pkg/proto"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/paths"
	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-docappender/v2/docappendertest"
	"github.com/elastic/go-elasticsearch/v8/esutil"
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
	b := newBeat(t, "output.console.enabled: true\nname: my-custom-name", func(args RunnerParams) (Runner, error) {
		calls <- args
		return newNopRunner(args), nil
	})
	stop := runBeat(t, b)
	args := expectRunnerParams(t, calls)
	assert.NoError(t, stop())

	assert.Equal(t, "apm-server", args.Info.Beat)
	assert.Equal(t, version.VersionWithQualifier(), args.Info.Version)
	assert.True(t, args.Info.ElasticLicensed)
	assert.Equal(t, "my-custom-name", b.Beat.Info.Name)
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
		"name": "my-custom-name",
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

// TestLibbeatMetrics tests the mapping of go-docappender OTel
// metrics to legacy libbeat monitoring metrics.
func TestLibbeatMetrics(t *testing.T) {
	runnerParamsChan := make(chan RunnerParams, 1)
	beat := newBeat(t, "output.elasticsearch.enabled: true", func(args RunnerParams) (Runner, error) {
		runnerParamsChan <- args
		return runnerFunc(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}), nil
	})
	stop := runBeat(t, beat)
	defer func() { assert.NoError(t, stop()) }()
	args := <-runnerParamsChan

	var requestIndex atomic.Int64
	requestsChan := make(chan chan struct{})
	defer close(requestsChan)
	esClient := docappendertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case ch := <-requestsChan:
			select {
			case <-r.Context().Done():
				return
			case <-ch:
			}
		}
		_, result := docappendertest.DecodeBulkRequest(r)
		switch requestIndex.Add(1) {
		case 2:
			result.HasErrors = true
			result.Items[0]["create"] = esutil.BulkIndexerResponseItem{Status: 400}
		case 4:
			result.HasErrors = true
			result.Items[0]["create"] = esutil.BulkIndexerResponseItem{Status: 429}
		default:
			// success
		}
		json.NewEncoder(w).Encode(result)
	})
	appender, err := docappender.New(esClient, docappender.Config{
		MeterProvider: args.MeterProvider,
		FlushBytes:    1,
		Scaling: docappender.ScalingConfig{
			ActiveRatio:  10,
			IdleInterval: 100 * time.Millisecond,
			ScaleUp: docappender.ScaleActionConfig{
				Threshold: 1,
				CoolDown:  time.Minute,
			},
		},
	})
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, appender.Close(context.Background()))
	}()

	require.NoError(t, appender.Add(context.Background(), "index", strings.NewReader("{}")))
	require.NoError(t, appender.Add(context.Background(), "index", strings.NewReader("{}")))
	require.NoError(t, appender.Add(context.Background(), "index", strings.NewReader("{}")))
	require.NoError(t, appender.Add(context.Background(), "index", strings.NewReader("{}")))

	libbeatRegistry := monitoring.Default.GetRegistry("libbeat")
	snapshot := monitoring.CollectStructSnapshot(libbeatRegistry, monitoring.Full, false)
	assert.Equal(t, map[string]any{
		"output": map[string]any{
			"type": "elasticsearch",
			"events": map[string]any{
				"active": int64(4),
				"total":  int64(4),
			},
		},
		"pipeline": map[string]any{
			"events": map[string]any{
				"total": int64(4),
			},
		},
	}, snapshot)

	assert.Eventually(t, func() bool {
		return appender.Stats().IndexersActive > 1
	}, 10*time.Second, 50*time.Millisecond)

	for i := 0; i < 4; i++ {
		unblockRequest := make(chan struct{})
		requestsChan <- unblockRequest
		unblockRequest <- struct{}{}
	}

	assert.Eventually(t, func() bool {
		snapshot = monitoring.CollectStructSnapshot(libbeatRegistry, monitoring.Full, false)
		output := snapshot["output"].(map[string]any)
		events := output["events"].(map[string]any)
		return events["active"] == int64(0)
	}, 10*time.Second, 100*time.Millisecond)
	assert.Equal(t, map[string]any{
		"output": map[string]any{
			"type": "elasticsearch",
			"events": map[string]any{
				"acked":   int64(2),
				"failed":  int64(2),
				"toomany": int64(1),
				"active":  int64(0),
				"total":   int64(4),
				"batches": int64(4),
			},
			"write": map[string]any{
				"bytes": int64(132),
			},
		},
		"pipeline": map[string]any{
			"events": map[string]any{
				"total": int64(4),
			},
		},
	}, snapshot)

	snapshot = monitoring.CollectStructSnapshot(monitoring.Default.GetRegistry("output"), monitoring.Full, false)
	assert.Equal(t, map[string]any{
		"elasticsearch": map[string]any{
			"bulk_requests": map[string]any{
				"available": int64(10),
				"completed": int64(4),
			},
			"indexers": map[string]any{
				"active":    int64(2),
				"created":   int64(1),
				"destroyed": int64(0),
			},
		},
	}, snapshot)
}

func TestUnmanagedOutputRequired(t *testing.T) {
	b := newBeat(t, "", func(args RunnerParams) (Runner, error) {
		panic("unreachable")
	})
	err := b.Run(context.Background())
	assert.EqualError(t, err, "no output defined, please define one under the output section")
}

func TestRunManager(t *testing.T) {
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

	err := b.Registry.GetInputList().Reload([]*reload.ConfigWithMeta{{
		Config: config.MustNewConfigFrom(`{
			"revision": 1,
			"apm-server.host": "localhost:1234"
		}`),
	}})
	assert.NoError(t, err)
	err = b.Registry.GetReloadableOutput().Reload(&reload.ConfigWithMeta{
		Config: config.MustNewConfigFrom(`{"console.enabled": true}`),
	})
	assert.NoError(t, err)

	expectRunnerParams(t, calls)
	err = b.Registry.GetReloadableAPM().Reload(&reload.ConfigWithMeta{
		Config: config.MustNewConfigFrom(`{"elastic.enabled": true, "elastic.environment": "testenv"}`),
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
		"instrumentation": map[string]interface{}{
			"enabled":     true,
			"environment": "testenv",
		},
	}, m)

	require.NotNil(t, manager.stopCallback)
	manager.stopCallback()
	assert.NoError(t, g.Wait())
	expectEvent(t, manager.stopped, "manager should have been stopped")
}

func TestRunManager_Reloader(t *testing.T) {
	// This test asserts that unit changes are reloaded correctly.

	finish := make(chan struct{})
	runCount := atomic.Int64{}
	stopCount := atomic.Int64{}

	registry := reload.NewRegistry()

	reloader, err := NewReloader(beat.Info{}, registry, func(p RunnerParams) (Runner, error) {
		return runnerFunc(func(ctx context.Context) error {
			revision, err := p.Config.Int("revision", -1)
			require.NoError(t, err)
			if revision == 2 {
				close(finish)
			}
			runCount.Add(1)
			<-ctx.Done()
			stopCount.Add(1)
			return nil
		}), nil
	}, nil, nil)
	require.NoError(t, err)

	agentInfo := &proto.AgentInfo{
		Id:       "elastic-agent-id",
		Version:  version.VersionWithQualifier(),
		Snapshot: true,
	}
	srv := integration.NewMockServer([]*proto.CheckinExpected{
		{
			AgentInfo: agentInfo,
			Units: []*proto.UnitExpected{
				{
					Id:             "output-unit",
					Type:           proto.UnitType_OUTPUT,
					ConfigStateIdx: 1,
					Config: &proto.UnitExpectedConfig{
						Id:   "default",
						Type: "elasticsearch",
						Name: "elasticsearch",
					},
					State:    proto.State_HEALTHY,
					LogLevel: proto.UnitLogLevel_INFO,
				},
				{
					Id:             "input-unit-1",
					Type:           proto.UnitType_INPUT,
					ConfigStateIdx: 1,
					Config: &proto.UnitExpectedConfig{
						Id:   "elastic-apm",
						Type: "apm",
						Name: "Elastic APM",
						Streams: []*proto.Stream{
							{
								Id: "elastic-apm",
								Source: integration.RequireNewStruct(t, map[string]interface{}{
									"revision": 1,
								}),
							},
						},
					},
					State:    proto.State_HEALTHY,
					LogLevel: proto.UnitLogLevel_INFO,
				},
			},
			Features:    nil,
			FeaturesIdx: 1,
		},
		{
			AgentInfo: agentInfo,
			Units: []*proto.UnitExpected{
				{
					Id:             "output-unit",
					Type:           proto.UnitType_OUTPUT,
					ConfigStateIdx: 1,
					State:          proto.State_HEALTHY,
					LogLevel:       proto.UnitLogLevel_INFO,
				},
				{
					Id:             "elastic-apm",
					Type:           proto.UnitType_INPUT,
					ConfigStateIdx: 2,
					Config: &proto.UnitExpectedConfig{
						Id:   "elastic-apm",
						Type: "apm",
						Name: "Elastic APM",
						Streams: []*proto.Stream{
							{
								Id: "elastic-apm",
								Source: integration.RequireNewStruct(t, map[string]interface{}{
									"revision": 2,
								}),
							},
						},
					},
					State:    proto.State_HEALTHY,
					LogLevel: proto.UnitLogLevel_INFO,
				},
			},
			Features:    nil,
			FeaturesIdx: 1,
		},
	},
		nil,
		500*time.Millisecond,
	)
	require.NoError(t, srv.Start())
	defer srv.Stop()

	client := client.NewV2(
		fmt.Sprintf(":%d", srv.Port),
		"",
		client.VersionInfo{},
		client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())))
	manager, err := xpacklbmanagement.NewV2AgentManagerWithClient(&xpacklbmanagement.Config{
		Enabled: true,
	}, registry, client)
	require.NoError(t, err)

	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-finish
		cancel()
	}()
	err = reloader.Run(ctx)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return runCount.Load() == 2 && stopCount.Load() == 2
	}, 2*time.Second, 50*time.Millisecond)
}

func TestRunManager_Reloader_newRunnerError(t *testing.T) {
	// This test asserts that any errors when creating runner inside reloader, e.g. config parsing error,
	// will cause the unit to fail.

	inputFailedMsg := make(chan string)

	registry := reload.NewRegistry()

	_, err := NewReloader(beat.Info{}, registry, func(_ RunnerParams) (Runner, error) {
		return nil, errors.New("newRunner error")
	}, nil, nil)
	require.NoError(t, err)

	onObserved := func(observed *proto.CheckinObserved, currentIdx int) {
		for _, unit := range observed.GetUnits() {
			if unit.GetId() == "input-unit-1" && unit.GetState() == proto.State_FAILED {
				inputFailedMsg <- unit.GetMessage()
			}
		}
	}
	agentInfo := &proto.AgentInfo{
		Id:       "elastic-agent-id",
		Version:  version.VersionWithQualifier(),
		Snapshot: true,
	}
	srv := integration.NewMockServer([]*proto.CheckinExpected{
		{
			AgentInfo: agentInfo,
			Units: []*proto.UnitExpected{
				{
					Id:             "output-unit",
					Type:           proto.UnitType_OUTPUT,
					ConfigStateIdx: 1,
					Config: &proto.UnitExpectedConfig{
						Id:   "default",
						Type: "elasticsearch",
						Name: "elasticsearch",
					},
					State:    proto.State_HEALTHY,
					LogLevel: proto.UnitLogLevel_INFO,
				},
				{
					Id:             "input-unit-1",
					Type:           proto.UnitType_INPUT,
					ConfigStateIdx: 1,
					Config: &proto.UnitExpectedConfig{
						Id:   "elastic-apm",
						Type: "apm",
						Name: "Elastic APM",
						Streams: []*proto.Stream{
							{
								Id: "elastic-apm",
								Source: integration.RequireNewStruct(t, map[string]interface{}{
									"revision": 1,
								}),
							},
						},
					},
					State:    proto.State_HEALTHY,
					LogLevel: proto.UnitLogLevel_INFO,
				},
			},
			Features:    nil,
			FeaturesIdx: 1,
		},
	},
		onObserved,
		500*time.Millisecond,
	)
	require.NoError(t, srv.Start())
	defer srv.Stop()

	client := client.NewV2(
		fmt.Sprintf(":%d", srv.Port),
		"",
		client.VersionInfo{},
		client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())))
	manager, err := xpacklbmanagement.NewV2AgentManagerWithClient(&xpacklbmanagement.Config{
		Enabled: true,
	}, registry, client)
	require.NoError(t, err)

	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	assert.Equal(t, "failed to load input config: newRunner error", <-inputFailedMsg)
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
	for _, name := range []string{"system", "beat", "libbeat", "apm-server", "output"} {
		registry := monitoring.Default.GetRegistry(name)
		if registry != nil {
			registry.Clear()
		}
	}
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
