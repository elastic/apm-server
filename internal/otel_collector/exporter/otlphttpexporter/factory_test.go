// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlphttpexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/testutil"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
	ocfg, ok := factory.CreateDefaultConfig().(*Config)
	assert.True(t, ok)
	assert.Equal(t, ocfg.HTTPClientSettings.Endpoint, "")
	assert.Equal(t, ocfg.HTTPClientSettings.Timeout, 30*time.Second, "default timeout is 30 second")
	assert.Equal(t, ocfg.RetrySettings.Enabled, true, "default retry is enabled")
	assert.Equal(t, ocfg.RetrySettings.MaxElapsedTime, 300*time.Second, "default retry MaxElapsedTime")
	assert.Equal(t, ocfg.RetrySettings.InitialInterval, 5*time.Second, "default retry InitialInterval")
	assert.Equal(t, ocfg.RetrySettings.MaxInterval, 30*time.Second, "default retry MaxInterval")
	assert.Equal(t, ocfg.QueueSettings.Enabled, true, "default sending queue is enabled")
}

func TestCreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "http://" + testutil.GetAvailableLocalAddress(t)

	creationParams := component.ExporterCreateParams{Logger: zap.NewNop()}
	oexp, err := factory.CreateMetricsExporter(context.Background(), creationParams, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)
}

func TestCreateTraceExporter(t *testing.T) {
	endpoint := "http://" + testutil.GetAvailableLocalAddress(t)

	tests := []struct {
		name     string
		config   Config
		mustFail bool
	}{
		{
			name: "NoEndpoint",
			config: Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "",
				},
			},
			mustFail: true,
		},
		{
			name: "UseSecure",
			config: Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: endpoint,
					TLSSetting: configtls.TLSClientSetting{
						Insecure: false,
					},
				},
			},
		},
		{
			name: "Headers",
			config: Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: endpoint,
					Headers: map[string]string{
						"hdr1": "val1",
						"hdr2": "val2",
					},
				},
			},
		},
		{
			name: "CaCert",
			config: Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: endpoint,
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "testdata/test_cert.pem",
						},
					},
				},
			},
		},
		{
			name: "CertPemFileError",
			config: Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: endpoint,
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "nosuchfile",
						},
					},
				},
			},
			mustFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			creationParams := component.ExporterCreateParams{Logger: zap.NewNop()}
			consumer, err := factory.CreateTracesExporter(context.Background(), creationParams, &tt.config)

			if tt.mustFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, consumer)

				err = consumer.Shutdown(context.Background())
				if err != nil {
					// Since the endpoint of OTLP exporter doesn't actually exist,
					// exporter may already stop because it cannot connect.
					assert.Equal(t, err.Error(), "rpc error: code = Canceled desc = grpc: the client connection is closing")
				}
			}
		})
	}
}

func TestCreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "http://" + testutil.GetAvailableLocalAddress(t)

	creationParams := component.ExporterCreateParams{Logger: zap.NewNop()}
	oexp, err := factory.CreateLogsExporter(context.Background(), creationParams, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)
}
