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

package kibana

import (
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/backoff"
	"github.com/elastic/beats/libbeat/kibana"
	"github.com/elastic/beats/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
)

const (
	initBackoff = time.Second
	maxBackoff  = 30 * time.Second
)

var errNotConnected = errors.New("unable to retrieve connection to Kibana")

// Client provides an interface for Kibana Clients
type Client interface {
	// Send tries to send request to Kibana and returns unparsed response
	Send(string, string, url.Values, http.Header, io.Reader) (*http.Response, error)
	// GetVersion returns Kibana version or an error
	GetVersion() (common.Version, error)
	// Connected indicates whether or not a connection to Kibana has been established
	Connected() bool
	// SupportsVersion compares given version to version of connected Kibana instance
	SupportsVersion(*common.Version, bool) (bool, error)
}

// ConnectingClient implements Client interface
type ConnectingClient struct {
	m      sync.RWMutex
	client *kibana.Client
	cfg    *kibana.ClientConfig
}

// NewConnectingClient returns instance of ConnectingClient and starts a background routine trying to connect
// to configured Kibana instance, using JitterBackoff for establishing connection.
func NewConnectingClient(cfg *kibana.ClientConfig) Client {
	c := &ConnectingClient{cfg: cfg}
	go func() {
		log := logp.NewLogger(logs.Kibana)
		done := make(chan struct{})
		jitterBackoff := backoff.NewEqualJitterBackoff(done, initBackoff, maxBackoff)
		for c.client == nil {
			log.Debug("Trying to obtain connection to Kibana.")
			err := c.connect()
			if err != nil {
				log.Errorf("failed to obtain connection to Kibana: %s", err.Error())
			}
			backoff.WaitOnError(jitterBackoff, err)
		}
		log.Info("Successfully obtained connection to Kibana.")
	}()

	return c
}

// Send tries to send a request to Kibana via established connection and returns unparsed response
// If no connection is established an error is returned
func (c *ConnectingClient) Send(method, extraPath string, params url.Values,
	headers http.Header, body io.Reader) (*http.Response, error) {
	c.m.RLock()
	defer c.m.RUnlock()
	if c.client == nil {
		return nil, errNotConnected
	}
	return c.client.Send(method, extraPath, params, headers, body)
}

// GetVersion returns Kibana version or an error
// If no connection is established an error is returned
func (c *ConnectingClient) GetVersion() (common.Version, error) {
	c.m.RLock()
	defer c.m.RUnlock()
	if c.client == nil {
		return common.Version{}, errNotConnected
	}
	return c.client.GetVersion(), nil
}

// Connected checks if a connection has been established
func (c *ConnectingClient) Connected() bool {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.client != nil
}

// SupportsVersion checks if connected Kibana instance is compatible to given version
// If no connection is established an error is returned
func (c *ConnectingClient) SupportsVersion(v *common.Version, retry bool) (bool, error) {
	log := logp.NewLogger(logs.Kibana)
	c.m.RLock()
	if c.client == nil && !retry {
		c.m.RUnlock()
		return false, errNotConnected
	}
	upToDate := c.client != nil && v.LessThanOrEqual(false, &c.client.Version)
	c.m.RUnlock()
	if !retry || upToDate {
		return upToDate, nil
	}
	client, err := kibana.NewClientWithConfig(c.cfg)
	if err != nil {
		log.Errorf("failed to obtain connection to Kibana: %s", err.Error())
		return upToDate, err
	}
	c.m.Lock()
	c.client = client
	c.m.Unlock()
	return c.SupportsVersion(v, false)
}

func (c *ConnectingClient) connect() error {
	if c.client != nil {
		return nil
	}
	c.m.Lock()
	defer c.m.Unlock()
	if c.client != nil {
		return nil
	}
	client, err := kibana.NewClientWithConfig(c.cfg)
	if err != nil {
		return err
	}
	c.client = client
	return nil
}
