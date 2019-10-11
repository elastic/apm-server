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

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/libbeat/outputs/transport"

	goelasticsearch "github.com/elastic/go-elasticsearch/v8"
)

const (
	prefixHttp       = "http"
	prefixHttpScheme = prefixHttp + "://"
	esDefaultPort    = 9200
)

var (
	invalidHosts = errors.New("invalid host configuration, must at least contain one hostname")
)

type Config struct {
	Hosts        hosts             `config:"hosts" validate:"required"`
	Protocol     string            `config:"protocol"`
	Path         string            `config:"path"`
	ProxyURL     string            `config:"proxy_url"`
	ProxyDisable bool              `config:"proxy_disable"`
	Timeout      time.Duration     `config:"timeout"`
	TLS          *tlscommon.Config `config:"ssl"`
}
type hosts []string

func Client(cfg *common.Config) (*goelasticsearch.Client, error) {
	// Ignored configurations
	// - compressionLevel not supported
	// - parameters
	// - worker, index, indices, pipeline, bulkMaxSize
	// - maxRetries, backoff, loadbalance, header: currently not supported in go-elasticsearch client
	// - timeout might be retrieved from configured apm-server request timeout

	//following logic is partially copied from libbeat

	if cfg == nil {
		return nil, nil
	}

	var config Config
	if err := cfg.Unpack(&config); err != nil {
		return nil, err
	}
	var err error
	proxy, err := httpProxyUrl(&config)
	if err != nil {
		return nil, err
	}

	addresses, err := addresses(&config)
	if err != nil {
		return nil, err
	}

	dialer, tlsDialer, err := dialer(&config)
	if err != nil {
		return nil, err
	}

	return goelasticsearch.NewClient(goelasticsearch.Config{
		Addresses: addresses,
		Transport: &http.Transport{
			Proxy:   proxy,
			Dial:    dialer.Dial,
			DialTLS: tlsDialer.Dial,
		},
	})
}

func (h hosts) Validate() error {
	if len(h) == 0 {
		return invalidHosts
	}
	return nil
}

func httpProxyUrl(cfg *Config) (func(*http.Request) (*url.URL, error), error) {
	if cfg.ProxyDisable {
		return nil, nil
	}

	if cfg.ProxyURL == "" {
		return http.ProxyFromEnvironment, nil
	}

	proxyStr := cfg.ProxyURL
	if !strings.HasPrefix(proxyStr, prefixHttp) {
		proxyStr = prefixHttpScheme + proxyStr
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
		address, err := common.MakeURL(cfg.Protocol, cfg.Path, host, esDefaultPort)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}
	return addresses, nil
}

func dialer(cfg *Config) (transport.Dialer, transport.Dialer, error) {
	var tlsConfig *tlscommon.TLSConfig
	var err error
	if cfg.TLS.IsEnabled() {
		if tlsConfig, err = tlscommon.LoadTLSConfig(cfg.TLS); err != nil {
			return nil, nil, err
		}
	}

	dialer := transport.NetDialer(cfg.Timeout)
	tlsDialer, err := transport.TLSDialer(dialer, tlsConfig, cfg.Timeout)
	return dialer, tlsDialer, err
}
