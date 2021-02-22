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

import (
	"crypto/tls"

	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
)

const (
	defaultJaegerGRPCHost = "localhost:14250"
	defaultJaegerHTTPHost = "localhost:14268"
)

// JaegerConfig holds configuration for Jaeger span collection.
type JaegerConfig struct {
	GRPC JaegerGRPCConfig `config:"grpc"`
	HTTP JaegerHTTPConfig `config:"http"`
}

// JaegerGRPCConfig holds configuration for the Jaeger gRPC server.
type JaegerGRPCConfig struct {
	AuthTag string      `config:"auth_tag"`
	Enabled bool        `config:"enabled"`
	Host    string      `config:"host"`
	TLS     *tls.Config `config:"-"`
}

// JaegerHTTPConfig holds configuration for the Jaeger HTTP server.
type JaegerHTTPConfig struct {
	Enabled bool   `config:"enabled"`
	Host    string `config:"host"`
}

func (c *JaegerConfig) setup(cfg *Config) error {
	if cfg.TLS == nil || !cfg.TLS.IsEnabled() {
		return nil
	}
	if c.GRPC.Enabled {
		tlsServerConfig, err := tlscommon.LoadTLSServerConfig(cfg.TLS)
		if err != nil {
			return err
		}
		c.GRPC.TLS = tlsServerConfig.BuildServerConfig(c.GRPC.Host)
	}
	return nil
}

func defaultJaeger() JaegerConfig {
	return JaegerConfig{
		GRPC: JaegerGRPCConfig{
			Enabled: false,
			Host:    defaultJaegerGRPCHost,
		},
		HTTP: JaegerHTTPConfig{
			Enabled: false,
			Host:    defaultJaegerHTTPHost,
		},
	}
}
