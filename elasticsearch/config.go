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

package elasticsearch

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/transport"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
)

const (
	prefixHTTP       = "http"
	prefixHTTPSchema = prefixHTTP + "://"
	defaultESPort    = 9200
)

var (
	errInvalidHosts     = errors.New("`Hosts` must at least contain one hostname")
	errConfigMissing    = errors.New("config missing")
	esConnectionTimeout = 5 * time.Second
)

// Config holds all configurable fields that are used to create a Client
type Config struct {
	Hosts        Hosts             `config:"hosts" validate:"required"`
	Protocol     string            `config:"protocol"`
	Path         string            `config:"path"`
	ProxyURL     string            `config:"proxy_url"`
	ProxyDisable bool              `config:"proxy_disable"`
	Timeout      time.Duration     `config:"timeout"`
	TLS          *tlscommon.Config `config:"ssl"`
	Username     string            `config:"username"`
	Password     string            `config:"password"`
	APIKey       string            `config:"api_key"`
	Headers      map[string]string `config:"headers"`
	MaxRetries   int               `config:"max_retries"`

	// CompressionLevel holds the gzip compression level used when bulk indexing
	// with modelindexer; it is otherwise ignored.
	CompressionLevel int `config:"compression_level" validate:"min=0, max=9"`

	EnableCompatibilityMode bool `config:"enable_compatibility_mode"`

	elasticsearch.Backoff `config:"backoff"`
}

// DefaultConfig returns a default config.
func DefaultConfig() *Config {
	return &Config{
		Hosts:      []string{"localhost:9200"},
		Protocol:   "http",
		Timeout:    esConnectionTimeout,
		MaxRetries: 3,
		Backoff:    DefaultBackoffConfig,
	}
}

// Hosts is an array of host strings and needs to have at least one entry
type Hosts []string

// Validate ensures Hosts instance has at least one entry
func (h Hosts) Validate() error {
	if len(h) == 0 {
		return errInvalidHosts
	}
	return nil
}

func httpProxyURL(cfg *Config) (func(*http.Request) (*url.URL, error), error) {
	if cfg.ProxyDisable {
		return nil, nil
	}

	if cfg.ProxyURL == "" {
		return http.ProxyFromEnvironment, nil
	}

	proxyStr := cfg.ProxyURL
	if !strings.HasPrefix(proxyStr, prefixHTTP) {
		proxyStr = prefixHTTPSchema + proxyStr
	}
	u, err := url.Parse(proxyStr)
	if err != nil {
		return nil, err
	}
	return http.ProxyURL(u), nil
}

func addresses(cfg *Config) ([]string, error) {
	var addresses []string
	for _, host := range cfg.Hosts {
		address, err := common.MakeURL(cfg.Protocol, cfg.Path, host, defaultESPort)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}
	return addresses, nil
}

// NewHTTPTransport returns a new net/http.Transport for cfg.
func NewHTTPTransport(cfg *Config) (*http.Transport, error) {
	proxy, err := httpProxyURL(cfg)
	if err != nil {
		return nil, err
	}

	var tlsConfig *tlscommon.TLSConfig
	if cfg.TLS.IsEnabled() {
		if tlsConfig, err = tlscommon.LoadTLSConfig(cfg.TLS); err != nil {
			return nil, err
		}
	}
	dialer := transport.NetDialer(cfg.Timeout)
	tlsDialer := transport.TLSDialer(dialer, tlsConfig, cfg.Timeout)
	return &http.Transport{
		Proxy:           proxy,
		Dial:            dialer.Dial,
		DialTLS:         tlsDialer.Dial,
		TLSClientConfig: tlsConfig.ToConfig(),
	}, nil
}
