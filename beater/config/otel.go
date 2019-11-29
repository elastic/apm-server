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

package config

const (
	defaultJaegerGRPCHost = "localhost:14250"
)

// OtelConfig bundles information for supported open-telemetry collectors
type OtelConfig struct {
	Jaeger JaegerConfig `config:"jaeger"`
}

// JaegerConfig holds configuration for jaeger collector
//TODO(simi): add TLS support
type JaegerConfig struct {
	Enabled bool       `config:"enabled"`
	GRPC    GRPCConfig `config:"grpc"`
}

// GRPCConfig bundles information around a grpc server
type GRPCConfig struct {
	Host string     `config:"host"`
	TLS  *TLSConfig `config:"tls"`
}

// TLSConfig bundles information for TLS communication
type TLSConfig struct {
	CertFile string `config:"cert_file"`
	KeyFile  string `config:"key_file"`
}

func defaultOtel() *OtelConfig {
	return &OtelConfig{
		Jaeger: JaegerConfig{
			Enabled: false,
			GRPC: GRPCConfig{
				Host: defaultJaegerGRPCHost,
			},
		},
	}
}
