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

package beater

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/instrumentation"
	"github.com/elastic/beats/v7/libbeat/logp"
)

type testBeater struct {
	*beater
	b     *beat.Beat
	logs  *observer.ObservedLogs
	runCh chan error

	listenAddr string
	baseURL    string
	client     *http.Client
}

func setupServer(t *testing.T, cfg *common.Config, beatConfig *beat.BeatConfig, events chan beat.Event) (*testBeater, error) {
	if testing.Short() {
		t.Skip("skipping server test")
	}
	apmBeat, cfg := newBeat(t, cfg, beatConfig, events)
	return setupBeater(t, apmBeat, cfg, beatConfig)
}

func newBeat(t *testing.T, cfg *common.Config, beatConfig *beat.BeatConfig, events chan beat.Event) (*beat.Beat, *common.Config) {
	info := beat.Info{
		Beat:        "test-apm-server",
		IndexPrefix: "test-apm-server",
		Version:     "1.2.3", // hard-coded to avoid changing approvals
		ID:          uuid.Must(uuid.FromString("fbba762a-14dd-412c-b7e9-b79f903eb492")),
	}

	combinedConfig := common.MustNewConfigFrom(map[string]interface{}{
		"host": "localhost:0",

		// Enable instrumentation so the profile endpoint is
		// available, but set the profiling interval to something
		// long enough that it won't kick in.
		"instrumentation": map[string]interface{}{
			"enabled": true,
			"profiling": map[string]interface{}{
				"cpu": map[string]interface{}{
					"enabled":  true,
					"interval": "360s",
				},
			},
		},
	})
	if cfg != nil {
		require.NoError(t, cfg.Unpack(combinedConfig))
	}

	var pub beat.Pipeline
	if events != nil {
		// capture events
		pubClient := newChanClientWith(events)
		pub = dummyPipeline(cfg, info, pubClient)
	} else {
		// don't capture events
		pub = dummyPipeline(cfg, info)
	}

	instrumentation, err := instrumentation.New(combinedConfig, info.Beat, info.Version)
	require.NoError(t, err)
	return &beat.Beat{
		Publisher:       pub,
		Info:            info,
		Config:          beatConfig,
		Instrumentation: instrumentation,
	}, combinedConfig
}

func setupBeater(
	t *testing.T,
	apmBeat *beat.Beat,
	ucfg *common.Config,
	beatConfig *beat.BeatConfig,
) (*testBeater, error) {
	tb, err := newTestBeater(t, apmBeat, ucfg, beatConfig)
	if err != nil {
		return nil, err
	}
	tb.start()

	listenAddr, err := tb.waitListenAddr(10 * time.Second)
	if err != nil {
		return nil, err
	}
	tb.initClient(tb.config, listenAddr)

	res, err := tb.client.Get(tb.baseURL)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	return tb, nil
}

func newTestBeater(
	t *testing.T,
	apmBeat *beat.Beat,
	ucfg *common.Config,
	beatConfig *beat.BeatConfig,
) (*testBeater, error) {

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	createBeater := NewCreator(CreatorParams{
		Logger: logger,
		WrapRunServer: func(runServer RunServerFunc) RunServerFunc {
			return func(ctx context.Context, args ServerParams) error {
				// Wrap the reporter so we can intercept the
				// onboarding doc, to extract the listen address.
				origReporter := args.Reporter
				args.Reporter = func(ctx context.Context, req publish.PendingReq) error {
					for _, tf := range req.Transformables {
						switch tf := tf.(type) {
						case *model.Transaction:
							// Add a label to test that everything
							// goes through the wrapped reporter.
							if tf.Labels == nil {
								tf.Labels = common.MapStr{}
							}
							tf.Labels["wrapped_reporter"] = true
						}
					}
					return origReporter(ctx, req)
				}
				return runServer(ctx, args)
			}
		},
	})

	beatBeater, err := createBeater(apmBeat, ucfg)
	if err != nil {
		return nil, err
	}
	require.NotNil(t, beatBeater)
	t.Cleanup(func() {
		beatBeater.Stop()
	})

	return &testBeater{
		beater: beatBeater.(*beater),
		b:      apmBeat,
		logs:   observedLogs,
		runCh:  make(chan error),
	}, nil
}

// start starts running a beater created with newTestBeater.
func (tb *testBeater) start() {
	go func() {
		tb.runCh <- tb.beater.Run(tb.b)
	}()
}

func (tb *testBeater) waitListenAddr(timeout time.Duration) (string, error) {
	deadline := time.After(timeout)
	for {
		for _, entry := range tb.logs.TakeAll() {
			const prefix = "Listening on: "
			if strings.HasPrefix(entry.Message, prefix) {
				listenAddr := entry.Message[len(prefix):]
				return listenAddr, nil
			}
		}
		select {
		case err := <-tb.runCh:
			if err != nil {
				return "", err
			}
			return "", errors.New("server exited cleanly without logging expected message")
		case <-deadline:
			return "", errors.New("timeout waiting for server to start listening")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (tb *testBeater) initClient(cfg *config.Config, listenAddr string) {
	tb.listenAddr = listenAddr
	transport := &http.Transport{}
	if parsed, err := url.Parse(cfg.Host); err == nil && parsed.Scheme == "unix" {
		transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", parsed.Path)
		}
		tb.baseURL = "http://test-apm-server/"
		tb.client = &http.Client{Transport: transport}
	} else {
		scheme := "http://"
		if cfg.TLS.IsEnabled() {
			scheme = "https://"
		}
		tb.baseURL = scheme + listenAddr
		tb.client = &http.Client{Transport: transport}
	}
}

func TestTransformConfigIndex(t *testing.T) {
	test := func(t *testing.T, indexPattern, expected string) {
		var requestPaths []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestPaths = append(requestPaths, r.URL.Path)
		}))
		defer srv.Close()

		cfg := config.DefaultConfig()
		cfg.RumConfig.Enabled = newBool(true)
		cfg.RumConfig.SourceMapping.ESConfig.Hosts = []string{srv.URL}
		if indexPattern != "" {
			cfg.RumConfig.SourceMapping.IndexPattern = indexPattern
		}

		transformConfig, err := newTransformConfig(beat.Info{Version: "1.2.3"}, cfg)
		require.NoError(t, err)
		require.NotNil(t, transformConfig.RUM.SourcemapStore)
		transformConfig.RUM.SourcemapStore.Added(context.Background(), "name", "version", "path")
		require.Len(t, requestPaths, 1)

		path := requestPaths[0]
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/_search")
		assert.Equal(t, expected, path)
	}
	t.Run("default-pattern", func(t *testing.T) { test(t, "", "apm-*-sourcemap*") })
	t.Run("with-observer-version", func(t *testing.T) { test(t, "blah-%{[observer.version]}-blah", "blah-1.2.3-blah") })
}

func TestTransformConfig(t *testing.T) {
	test := func(rumEnabled, sourcemapEnabled *bool, expectSourcemapStore bool) {
		cfg := config.DefaultConfig()
		cfg.RumConfig.Enabled = rumEnabled
		cfg.RumConfig.SourceMapping.Enabled = sourcemapEnabled
		transformConfig, err := newTransformConfig(beat.Info{Version: "1.2.3"}, cfg)
		require.NoError(t, err)
		if expectSourcemapStore {
			assert.NotNil(t, transformConfig.RUM.SourcemapStore)
		} else {
			assert.Nil(t, transformConfig.RUM.SourcemapStore)
		}
	}

	test(nil, nil, false)
	test(nil, newBool(false), false)
	test(nil, newBool(true), false)

	test(newBool(false), nil, false)
	test(newBool(false), newBool(false), false)
	test(newBool(false), newBool(true), false)

	test(newBool(true), nil, true) // sourcemap.enabled is true by default
	test(newBool(true), newBool(false), false)
	test(newBool(true), newBool(true), true)
}

func newBool(v bool) *bool {
	return &v
}
