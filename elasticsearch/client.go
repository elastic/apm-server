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
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"go.elastic.co/apm/module/apmelasticsearch"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/version"
	esv7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	esv8 "github.com/elastic/go-elasticsearch/v8"
)

// Client is an interface designed to abstract away version differences between elasticsearch clients
type Client interface {
	// Perform satisfies esapi.Transport
	Perform(*http.Request) (*http.Response, error)
	// TODO: deprecate
	SearchQuery(ctx context.Context, index string, body io.Reader) (int, io.ReadCloser, error)
}

type clientV8 struct {
	*esv8.Client
}

func (c clientV8) SearchQuery(ctx context.Context, index string, body io.Reader) (int, io.ReadCloser, error) {
	response, err := c.Search(
		c.Search.WithContext(ctx),
		c.Search.WithIndex(index),
		c.Search.WithBody(body),
		c.Search.WithTrackTotalHits(true),
		c.Search.WithPretty(),
	)
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}

type clientV7 struct {
	*esv7.Client
}

func (c clientV7) SearchQuery(ctx context.Context, index string, body io.Reader) (int, io.ReadCloser, error) {
	response, err := c.Search(
		c.Search.WithContext(ctx),
		c.Search.WithIndex(index),
		c.Search.WithBody(body),
		c.Search.WithTrackTotalHits(true),
		c.Search.WithPretty(),
	)
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}

// NewClient parses the given config and returns  a version-aware client as an interface
func NewClient(config *Config) (Client, error) {
	if config == nil {
		return nil, errConfigMissing
	}
	transport, addresses, headers, err := connectionConfig(config)
	if err != nil {
		return nil, err
	}
	return NewVersionedClient(config.APIKey, config.Username, config.Password, addresses, headers, transport)
}

// NewVersionedClient returns the right elasticsearch client for the current Stack version, as an interface
func NewVersionedClient(apikey, user, pwd string, addresses []string, headers http.Header, transport http.RoundTripper) (Client, error) {
	if apikey != "" {
		apikey = base64.StdEncoding.EncodeToString([]byte(apikey))
	}
	transport = apmelasticsearch.WrapRoundTripper(transport)
	version := common.MustNewVersion(version.GetDefaultVersion())
	if version.IsMajor(8) {
		c, err := newV8Client(apikey, user, pwd, addresses, headers, transport)
		return clientV8{c}, err
	}
	c, err := newV7Client(apikey, user, pwd, addresses, headers, transport)
	return clientV7{c}, err
}

func newV7Client(apikey, user, pwd string, addresses []string, headers http.Header, transport http.RoundTripper) (*esv7.Client, error) {
	return esv7.NewClient(esv7.Config{
		APIKey:    apikey,
		Username:  user,
		Password:  pwd,
		Addresses: addresses,
		Transport: transport,
		Header:    headers,
	})
}

func newV8Client(apikey, user, pwd string, addresses []string, headers http.Header, transport http.RoundTripper) (*esv8.Client, error) {
	return esv8.NewClient(esv8.Config{
		APIKey:    apikey,
		Username:  user,
		Password:  pwd,
		Addresses: addresses,
		Transport: transport,
		Header:    headers,
	})
}

func doRequest(ctx context.Context, transport esapi.Transport, req esapi.Request, out interface{}) error {
	resp, err := req.Do(ctx, transport)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return &Error{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			body:       string(bytes),
		}
	}
	if out != nil {
		err = json.NewDecoder(resp.Body).Decode(out)
	}
	return err
}

// Error holds the details for a failed Elasticsearch request.
//
// Error is only returned for request is serviced, and not when
// a client or network failure occurs.
type Error struct {
	// StatusCode holds the HTTP response status code.
	StatusCode int

	// Header holds the HTTP response headers.
	Header http.Header

	body string
}

func (e *Error) Error() string {
	if e.body != "" {
		return e.body
	}
	return http.StatusText(e.StatusCode)
}
