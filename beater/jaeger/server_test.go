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

package jaeger

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	jaegermodel "github.com/jaegertracing/jaeger/model"
	jaegerthriftconv "github.com/jaegertracing/jaeger/model/converter/thrift/jaeger"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	jaegerthrift "github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/apmtest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model"
)

func TestApprovals(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.JaegerConfig.GRPC.Enabled = true
	cfg.JaegerConfig.GRPC.Host = "localhost:0"
	cfg.JaegerConfig.HTTP.Enabled = true
	cfg.JaegerConfig.HTTP.Host = "localhost:0"

	for _, name := range []string{
		"batch_0", "batch_1",
	} {
		t.Run(name, func(t *testing.T) {
			tc := testcase{cfg: cfg, grpcDialOpts: []grpc.DialOption{grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			)}}
			tc.setup(t)
			defer tc.teardown(t)

			f := filepath.Join("..", "..", "testdata", "jaeger", name)
			data, err := ioutil.ReadFile(f + ".json")
			require.NoError(t, err)
			var request api_v2.PostSpansRequest
			require.NoError(t, json.Unmarshal(data, &request))

			require.NoError(t, tc.sendBatchGRPC(request.Batch))
			docs := beatertest.EncodeEventDocs(tc.events...)
			approvaltest.ApproveEventDocs(t, f, docs)

			tc.events = nil
			thriftBatch := &jaegerthrift.Batch{
				Process: thriftProcessFromModel(request.Batch.Process),
				Spans:   jaegerthriftconv.FromDomain(request.Batch.Spans),
			}
			require.NoError(t, tc.sendBatchHTTP(thriftBatch))
			docs = beatertest.EncodeEventDocs(tc.events...)
			approvaltest.ApproveEventDocs(t, f, docs)
		})
	}
}

func TestServerIntegration(t *testing.T) {
	insecureOpt := grpc.WithTransportCredentials(
		insecure.NewCredentials(),
	)
	for name, tc := range map[string]testcase{
		"default config": {
			cfg: config.DefaultConfig(),
		},
		"default config with Jaeger gRPC enabled": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.JaegerConfig.GRPC.Enabled = true
				cfg.JaegerConfig.GRPC.Host = "localhost:0"
				return cfg
			}(),
			grpcDialOpts: []grpc.DialOption{insecureOpt},
		},
		"default config with Jaeger gRPC and Kibana enabled": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.JaegerConfig.GRPC.Enabled = true
				cfg.JaegerConfig.GRPC.Host = "localhost:0"
				cfg.Kibana.Enabled = true
				cfg.Kibana.Host = "non-existing:1234"
				return cfg
			}(),
			grpcDialOpts: []grpc.DialOption{insecureOpt},
		},
		"default config with Jaeger HTTP enabled": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.JaegerConfig.HTTP.Enabled = true
				cfg.JaegerConfig.HTTP.Host = "localhost:0"
				return cfg
			}(),
		},
		"default config with Jaeger gRPC and HTTP enabled": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.JaegerConfig.GRPC.Enabled = true
				cfg.JaegerConfig.GRPC.Host = "localhost:0"
				cfg.JaegerConfig.HTTP.Enabled = true
				cfg.JaegerConfig.HTTP.Host = "localhost:0"
				return cfg
			}(),
			grpcDialOpts: []grpc.DialOption{insecureOpt},
		},
		"with Jaeger gRPC enabled and TLS disabled": {
			cfg: &config.Config{
				JaegerConfig: config.JaegerConfig{
					GRPC: config.JaegerGRPCConfig{
						Enabled: true,
						Host:    "localhost:0",
					},
				},
			},
			grpcDialOpts: []grpc.DialOption{insecureOpt},
		},
		"with Jaeger gRPC enabled and TLS enabled, client configured with CA cert": {
			cfg: &config.Config{
				JaegerConfig: config.JaegerConfig{
					GRPC: config.JaegerGRPCConfig{
						Enabled: true,
						Host:    "localhost:0",
						TLS: func() *tls.Config {
							enabledTrue := true
							serverConfig, err := tlscommon.LoadTLSServerConfig(&tlscommon.ServerConfig{
								Enabled: &enabledTrue,
								Certificate: tlscommon.CertificateConfig{
									Certificate: filepath.Join("..", "..", "testdata", "tls", "certificate.pem"),
									Key:         filepath.Join("..", "..", "testdata", "tls", "key.pem"),
								},
							})
							if err != nil {
								panic(err)
							}
							return serverConfig.BuildServerConfig("")
						}(),
					},
				},
			},
			grpcDialOpts: func() []grpc.DialOption {
				tls, err := credentials.NewClientTLSFromFile(filepath.Join("..", "..", "testdata", "tls", "ca.crt.pem"), "apm-server")
				require.NoError(t, err)
				return []grpc.DialOption{grpc.WithTransportCredentials(tls)}
			}(),
		},
		"with Jaeger gRPC enabled and TLS enabled, client fails without transport security": {
			cfg: &config.Config{
				JaegerConfig: config.JaegerConfig{
					GRPC: config.JaegerGRPCConfig{
						Enabled: true,
						Host:    "localhost:0",
						TLS: func() *tls.Config {
							enabledTrue := true
							serverConfig, err := tlscommon.LoadTLSServerConfig(&tlscommon.ServerConfig{
								Enabled: &enabledTrue,
								Certificate: tlscommon.CertificateConfig{
									Certificate: filepath.Join("..", "..", "testdata", "tls", "certificate.pem"),
									Key:         filepath.Join("..", "..", "testdata", "tls", "key.pem"),
								},
							})
							if err != nil {
								panic(err)
							}
							return serverConfig.BuildServerConfig("")
						}(),
					},
				},
			},
			grpcDialShouldFail: true,
		},
		"with Jaeger gRPC enabled and TLS enabled, client fails to verify server certificate": {
			cfg: &config.Config{
				JaegerConfig: config.JaegerConfig{
					GRPC: config.JaegerGRPCConfig{
						Enabled: true,
						Host:    "localhost:0",
						TLS: func() *tls.Config {
							enabledTrue := true
							serverConfig, err := tlscommon.LoadTLSServerConfig(&tlscommon.ServerConfig{
								Enabled: &enabledTrue,
								Certificate: tlscommon.CertificateConfig{
									Certificate: filepath.Join("..", "..", "testdata", "tls", "certificate.pem"),
									Key:         filepath.Join("..", "..", "testdata", "tls", "key.pem"),
								},
							})
							if err != nil {
								panic(err)
							}
							return serverConfig.BuildServerConfig("")
						}(),
					},
				},
			},
			// grpc.Dial returns immediately, and the connection happens in the background.
			// Therefore the dial won't fail, the subsequent PostSpans call will fail due
			// to the certificate verification failing.
			grpcSendSpanShouldFail: true,
			grpcDialOpts: func() []grpc.DialOption {
				return []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))}
			}(),
		},
		"secret token set but no auth_tag": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.AgentAuth.SecretToken = "hunter2"
				cfg.JaegerConfig.GRPC.Enabled = true
				cfg.JaegerConfig.GRPC.Host = "localhost:0"
				cfg.JaegerConfig.HTTP.Enabled = true
				cfg.JaegerConfig.HTTP.Host = "localhost:0"
				return cfg
			}(),
			grpcDialOpts: []grpc.DialOption{insecureOpt},
		},
		"secret token and auth_tag set, but no auth_tag sent by agent": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.AgentAuth.SecretToken = "hunter2"
				cfg.JaegerConfig.GRPC.Enabled = true
				cfg.JaegerConfig.GRPC.Host = "localhost:0"
				cfg.JaegerConfig.GRPC.AuthTag = "authorization"
				return cfg
			}(),
			grpcDialOpts:           []grpc.DialOption{insecureOpt},
			grpcSendSpanShouldFail: true, // unauthorized
		},
		"secret token and auth_tag set, auth_tag sent by agent": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.AgentAuth.SecretToken = "hunter2"
				cfg.JaegerConfig.GRPC.Enabled = true
				cfg.JaegerConfig.GRPC.Host = "localhost:0"
				cfg.JaegerConfig.GRPC.AuthTag = "authorization"
				return cfg
			}(),
			grpcDialOpts: []grpc.DialOption{insecureOpt},
			processTags: map[string]string{
				"authorization": "Bearer hunter2",
				"not":           "hotdog",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.setup(t)
			defer tc.teardown(t)

			if !tc.cfg.JaegerConfig.GRPC.Enabled && !tc.cfg.JaegerConfig.HTTP.Enabled {
				assert.Nil(t, tc.server)
				assert.Nil(t, tc.grpcClient)
				assert.Nil(t, tc.httpURL)
				return
			}

			var nevents, ntransactions int
			if tc.grpcClient != nil {
				// gRPC span collector
				err := tc.sendSpanGRPC()
				if tc.grpcSendSpanShouldFail {
					require.Error(t, err)
					require.Len(t, tc.events, nevents)
				} else {
					require.NoError(t, err)
					require.Len(t, tc.events, nevents+1)
					event := tc.events[nevents]
					for k := range tc.processTags {
						field := "labels." + k
						ok, err := event.Fields.HasKey(field)
						require.NoError(t, err)
						if k == tc.cfg.JaegerConfig.GRPC.AuthTag {
							assert.False(t, ok, field)
						} else {
							assert.True(t, ok, field)
						}
					}
					nevents++
				}
				if status.Code(err) != codes.Unavailable {
					tc.tracer.Flush(nil)
					transactions := tc.tracer.Payloads().Transactions
					require.Len(t, transactions, ntransactions+1)
					assert.Equal(t, "/jaeger.api_v2.CollectorService/PostSpans", transactions[ntransactions].Name)
					ntransactions++
				}

				// gRPC sampler
				resp, err := tc.sendSamplingGRPC()
				// no valid Kibana configuration as these are unit tests
				// for integration tests with Kibana see tests/system/test_jaeger.py
				require.Error(t, err)
				assert.Nil(t, resp)
				if status.Code(err) != codes.Unavailable {
					// a transaction event is recorded with the tracer
					tc.tracer.Flush(nil)
					transactions := tc.tracer.Payloads().Transactions
					require.Len(t, transactions, ntransactions+1)
					assert.Equal(t, "/jaeger.api_v2.SamplingManager/GetSamplingStrategy", transactions[ntransactions].Name)
					ntransactions++
				}
			}
			if tc.httpURL != nil {
				err := tc.sendSpanHTTP()
				require.NoError(t, err)
				require.Len(t, tc.events, nevents+1)
				nevents++

				tc.tracer.Flush(nil)
				transactions := tc.tracer.Payloads().Transactions
				require.Len(t, transactions, ntransactions+1)
				assert.Equal(t, "POST /api/traces", transactions[ntransactions].Name)
				ntransactions++
			}
		})
	}
}

type testcase struct {
	cfg                    *config.Config
	grpcDialOpts           []grpc.DialOption
	grpcDialShouldFail     bool
	grpcSendSpanShouldFail bool
	processTags            map[string]string

	events     []beat.Event
	server     *Server
	serverDone <-chan error
	grpcClient *grpc.ClientConn
	httpURL    *url.URL
	tracer     *apmtest.RecordingTracer
}

func (tc *testcase) setup(t *testing.T) {
	var batchProcessor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
		tc.events = append(tc.events, batch.Transform(ctx)...)
		return nil
	}

	var err error
	tc.tracer = apmtest.NewRecordingTracer()
	f := agentcfg.NewFetcher(tc.cfg)
	tc.server, err = NewServer(logp.NewLogger("jaeger"), tc.cfg, tc.tracer.Tracer, batchProcessor, f)
	require.NoError(t, err)
	if tc.server == nil {
		return
	}

	if tc.cfg.JaegerConfig.HTTP.Enabled {
		tc.httpURL = &url.URL{
			Scheme: "http",
			Host:   tc.server.http.listener.Addr().String(),
			Path:   "/api/traces",
		}
	} else {
		require.Nil(t, tc.server.http.server)
		require.Nil(t, tc.server.http.listener)
	}

	if tc.cfg.JaegerConfig.GRPC.Enabled {
		client, err := grpc.Dial(tc.server.grpc.listener.Addr().String(), tc.grpcDialOpts...)
		if tc.grpcDialShouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			tc.grpcClient = client
		}
	} else {
		require.Nil(t, tc.server.grpc.server)
		require.Nil(t, tc.server.grpc.listener)
	}

	serverDone := make(chan error, 1)
	tc.serverDone = serverDone
	go func() { serverDone <- tc.server.Serve() }()
}

func (tc *testcase) teardown(t *testing.T) {
	if tc.serverDone != nil {
		tc.server.Stop()
		err := <-tc.serverDone
		// grpc.ErrServerStopped may be returned by Serve
		// in the tests when Stop is called before Serve.
		if err != grpc.ErrServerStopped {
			require.NoError(t, err)
		}
	}
	if tc.grpcClient != nil {
		err := tc.sendSpanGRPC()
		require.Error(t, err) // server is closed
		tc.grpcClient.Close()
	}
	if tc.httpURL != nil {
		err := tc.sendSpanHTTP()
		require.Error(t, err) // server is closed
	}
	if tc.tracer != nil {
		tc.tracer.Close()
	}
}

func (tc *testcase) sendSpanGRPC() error {
	var tags []jaegermodel.KeyValue
	for k, v := range tc.processTags {
		tags = append(tags, jaegermodel.KeyValue{
			Key:   k,
			VStr:  v,
			VType: jaegermodel.ValueType_STRING,
		})
	}
	return tc.sendBatchGRPC(jaegermodel.Batch{
		Process: &jaegermodel.Process{Tags: tags},
		Spans: []*jaegermodel.Span{{
			TraceID: jaegermodel.NewTraceID(123, 456),
			SpanID:  jaegermodel.NewSpanID(789),
		}},
	})
}

func (tc *testcase) sendBatchGRPC(batch jaegermodel.Batch) error {
	client := api_v2.NewCollectorServiceClient(tc.grpcClient)
	_, err := client.PostSpans(context.Background(), &api_v2.PostSpansRequest{Batch: batch})
	return err
}

func (tc *testcase) sendSamplingGRPC() (*api_v2.SamplingStrategyResponse, error) {
	client := api_v2.NewSamplingManagerClient(tc.grpcClient)
	return client.GetSamplingStrategy(context.Background(), &api_v2.SamplingStrategyParameters{ServiceName: "xyz"})
}

func (tc *testcase) sendSpanHTTP() error {
	var tags []*jaegerthrift.Tag
	for k, v := range tc.processTags {
		v := v
		tags = append(tags, &jaegerthrift.Tag{
			Key:   k,
			VStr:  &v,
			VType: jaegerthrift.TagType_STRING,
		})
	}
	return tc.sendBatchHTTP(&jaegerthrift.Batch{
		Process: &jaegerthrift.Process{ServiceName: "whatever", Tags: tags},
		Spans:   []*jaegerthrift.Span{{TraceIdHigh: 123, TraceIdLow: 456, SpanId: 789}},
	})
}

func (tc *testcase) sendBatchHTTP(batch *jaegerthrift.Batch) error {
	body := encodeThriftBatch(batch)
	resp, err := http.Post(tc.httpURL.String(), "application/x-thrift", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("expected status %d, got %d", http.StatusAccepted, resp.StatusCode)
	}
	return nil
}

func thriftProcessFromModel(in *jaegermodel.Process) *jaegerthrift.Process {
	out := &jaegerthrift.Process{ServiceName: in.ServiceName}
	out.Tags = make([]*jaegerthrift.Tag, len(in.Tags))
	for i, kv := range in.Tags {
		kv := kv // copy for pointer refs
		tag := &jaegerthrift.Tag{Key: kv.Key, VType: jaegerthrift.TagType(kv.VType)}
		switch kv.VType {
		case jaegermodel.ValueType_STRING:
			tag.VStr = &kv.VStr
		case jaegermodel.ValueType_BOOL:
			tag.VBool = &kv.VBool
		case jaegermodel.ValueType_INT64:
			tag.VLong = &kv.VInt64
		case jaegermodel.ValueType_FLOAT64:
			tag.VDouble = &kv.VFloat64
		case jaegermodel.ValueType_BINARY:
			tag.VBinary = kv.VBinary
		}
		out.Tags[i] = tag
	}
	return out
}
