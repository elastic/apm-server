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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/paths"
	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-docappender/v2/docappendertest"
)

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
			result.Items[0]["create"] = docappendertest.BulkIndexerResponseItem{Status: 400}
		case 4:
			result.HasErrors = true
			result.Items[0]["create"] = docappendertest.BulkIndexerResponseItem{Status: 429}
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

	const totalRequests = 4

	for range totalRequests {
		require.NoError(t, appender.Add(context.Background(), "index", strings.NewReader("{}")))
	}

	statsRegistry := beat.Monitoring.StatsRegistry()
	libbeatRegistry := statsRegistry.GetRegistry("libbeat")
	snapshot := monitoring.CollectStructSnapshot(libbeatRegistry, monitoring.Full, false)
	assert.Equal(t, map[string]any{
		"output": map[string]any{
			"type": "elasticsearch",
			"events": map[string]any{
				"active": int64(totalRequests),
				"total":  int64(totalRequests),
			},
		},
		"pipeline": map[string]any{
			"events": map[string]any{
				"total": int64(totalRequests),
			},
		},
	}, snapshot)

	assert.Eventually(t, func() bool {
		return appender.IndexersActive() > 1
	}, 10*time.Second, 50*time.Millisecond)

	for range totalRequests {
		unblockRequest := make(chan struct{})
		requestsChan <- unblockRequest
		unblockRequest <- struct{}{}
	}

	assert.Eventually(t, func() bool {
		snapshot = monitoring.CollectStructSnapshot(libbeatRegistry, monitoring.Full, false)
		output := snapshot["output"].(map[string]any)
		events := output["events"].(map[string]any)
		return events["active"] == int64(0) && events["batches"] == int64(totalRequests)
	}, 10*time.Second, 100*time.Millisecond)
	assert.Equal(t, map[string]any{
		"output": map[string]any{
			"type": "elasticsearch",
			"events": map[string]any{
				"acked":   int64(2),
				"failed":  int64(2),
				"toomany": int64(1),
				"active":  int64(0),
				"total":   int64(totalRequests),
				"batches": int64(totalRequests),
			},
			"write": map[string]any{
				"bytes": int64(132),
			},
		},
		"pipeline": map[string]any{
			"events": map[string]any{
				"total": int64(totalRequests),
			},
		},
	}, snapshot)

	snapshot = monitoring.CollectStructSnapshot(statsRegistry.GetRegistry("output"), monitoring.Full, false)
	assert.Equal(t, map[string]any{
		"elasticsearch": map[string]any{
			"bulk_requests": map[string]any{
				"available": int64(10),
				"completed": int64(totalRequests),
			},
			"indexers": map[string]any{
				"active":    int64(2),
				"created":   int64(1),
				"destroyed": int64(0),
			},
		},
	}, snapshot)
}

// TestAddAPMServerMetrics tests basic functionality of the metrics collection and reporting
func TestAddAPMServerMetrics(t *testing.T) {
	r := monitoring.NewRegistry()
	sm := metricdata.ScopeMetrics{
		Metrics: []metricdata.Metrics{
			{
				Name: "apm-server.foo.request",
				Data: metricdata.Sum[int64]{
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Value: 1,
						},
					},
				},
			},
			{
				Name: "apm-server.foo.response",
				Data: metricdata.Sum[int64]{
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Value: 1,
						},
					},
				},
			},
		},
	}

	monitoring.NewFunc(r, "apm-server", func(m monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()

		beatsMetrics := make(map[string]any)
		addAPMServerMetricsToMap(beatsMetrics, sm.Metrics)
		reportOnKey(v, beatsMetrics)
	})

	snapshot := monitoring.CollectStructSnapshot(r, monitoring.Full, false)
	assert.Equal(t, map[string]any{
		"foo": map[string]any{
			"request":  int64(1),
			"response": int64(1),
		},
	}, snapshot["apm-server"])
}

func TestMonitoringApmServer(t *testing.T) {
	b := newNopBeat(t, "")
	b.registerStatsMetrics()

	// add metrics similar to lsm_size in storage_manager.go and events.processed in processor.go
	meter := b.meterProvider.Meter("github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage")
	lsmSizeGauge, _ := meter.Int64Gauge("apm-server.sampling.tail.storage.lsm_size")
	lsmSizeGauge.Record(context.Background(), 123)

	meter2 := b.meterProvider.Meter("github.com/elastic/apm-server/x-pack/apm-server/sampling")
	processedCounter, _ := meter2.Int64Counter("apm-server.sampling.tail.events.processed")
	processedCounter.Add(context.Background(), 456)

	meter3 := b.meterProvider.Meter("github.com/elastic/apm-server/x-pack/apm-server/foo")
	otherCounter, _ := meter3.Int64Counter("apm-server.sampling.foo.request")
	otherCounter.Add(context.Background(), 1)

	// collect metrics
	snapshot := monitoring.CollectStructSnapshot(b.Monitoring.StatsRegistry(), monitoring.Full, false)

	// assert that the snapshot contains data for all scoped metrics
	// with the same metric name prefix 'apm-server.sampling'
	assert.Equal(t, map[string]any{
		"apm-server": map[string]any{
			"sampling": map[string]any{
				"foo": map[string]any{
					"request": int64(1),
				},
				"tail": map[string]any{
					"storage": map[string]any{
						"lsm_size": int64(123),
					},
					"events": map[string]any{
						"processed": int64(456),
					},
				},
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
	management.SetManagerFactory(func(*config.C, *reload.Registry, *logp.Logger) (management.Manager, error) {
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
	expectedRun := 2
	expectedStop := 2
	success := make(chan struct{})

	registry := reload.NewRegistry()

	reloader, err := NewReloader(beat.Info{
		Logger: logptest.NewTestingLogger(t, "beat"),
	}, registry, func(p RunnerParams) (Runner, error) {
		return runnerFunc(func(ctx context.Context) error {
			revision, err := p.Config.Int("revision", -1)
			require.NoError(t, err)
			if revision == 2 {
				close(finish)
			}
			newRun := runCount.Add(1)
			<-ctx.Done()
			newStop := stopCount.Add(1)
			if newRun == int64(expectedRun) && newStop == int64(expectedStop) {
				close(success)
			}
			return nil
		}), nil
	}, nil, nil, nil, beat.NewMonitoring(), nil)
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
		10*time.Millisecond,
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
	}, registry, client, logptest.NewTestingLogger(t, "manager"), xpacklbmanagement.WithChangeDebounce(0))
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

	select {
	case <-success:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for success")
	}
}

func TestRunManager_Reloader_newRunnerError(t *testing.T) {
	// This test asserts that any errors when creating runner inside reloader, e.g. config parsing error,
	// will cause the unit to fail.

	inputFailedMsg := make(chan string)

	registry := reload.NewRegistry()

	_, err := NewReloader(beat.Info{
		Logger: logptest.NewTestingLogger(t, "beat"),
	}, registry, func(_ RunnerParams) (Runner, error) {
		return nil, errors.New("newRunner error")
	}, nil, nil, nil, beat.NewMonitoring(), nil)
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
	}, registry, client, logptest.NewTestingLogger(t, "manager"), xpacklbmanagement.WithChangeDebounce(0))
	require.NoError(t, err)

	err = manager.Start()
	require.NoError(t, err)
	defer manager.Stop()

	select {
	case msg := <-inputFailedMsg:
		assert.Equal(t, "failed to load input config: newRunner error", msg)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for input failed msg")
	}
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
	initCfgfile(t, configYAML)
	beat, err := NewBeat(BeatParams{
		NewRunner:       newRunner,
		ElasticLicensed: true,
		Logger:          logptest.NewTestingLogger(t, ""),
	})
	require.NoError(t, err)
	return beat
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
