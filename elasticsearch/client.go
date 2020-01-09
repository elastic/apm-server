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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/version"

	esv7 "github.com/elastic/go-elasticsearch/v7"

	esv8 "github.com/elastic/go-elasticsearch/v8"
)

// Client is an interface designed to abstract away version differences between elasticsearch clients
type Client interface {
	// TODO: deprecate
	// Search performs a query against the given index with the given body
	Search(index string, body io.Reader) (int, io.ReadCloser, error)
	// Makes a request with application/json Content-Type and Accept headers by default
	// pass/overwrite headers with "key: value" format
	JSONRequest(method, path string, body interface{}, headers ...string) JSONResponse
}

type clientV8 struct {
	v8 *esv8.Client
}

// Search satisfies the Client interface for version 8
func (c clientV8) Search(index string, body io.Reader) (int, io.ReadCloser, error) {
	response, err := c.v8.Search(
		c.v8.Search.WithContext(context.Background()),
		c.v8.Search.WithIndex(index),
		c.v8.Search.WithBody(body),
		c.v8.Search.WithTrackTotalHits(true),
		c.v8.Search.WithPretty(),
	)
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}

func (c clientV8) JSONRequest(method, path string, body interface{}, headers ...string) JSONResponse {
	req, err := makeRequest(method, path, body, headers...)
	if err != nil {
		return JSONResponse{nil, err}
	}
	return parseResponse(c.v8.Perform(req))
}

type clientV7 struct {
	v7 *esv7.Client
}

// Search satisfies the Client interface for version 7
func (c clientV7) Search(index string, body io.Reader) (int, io.ReadCloser, error) {
	response, err := c.v7.Search(
		c.v7.Search.WithContext(context.Background()),
		c.v7.Search.WithIndex(index),
		c.v7.Search.WithBody(body),
		c.v7.Search.WithTrackTotalHits(true),
		c.v7.Search.WithPretty(),
	)
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}

func (c clientV7) JSONRequest(method, path string, body interface{}, headers ...string) JSONResponse {
	req, err := makeRequest(method, path, body, headers...)
	if err != nil {
		return JSONResponse{nil, err}
	}
	return parseResponse(c.v7.Perform(req))
}

// NewClient parses the given config and returns  a version-aware client as an interface
func NewClient(config *Config) (Client, error) {
	if config == nil {
		return nil, errConfigMissing
	}
	transport, addresses, err := connectionConfig(config)
	if err != nil {
		return nil, err
	}
	return NewVersionedClient(config.APIKey, config.Username, config.Password, addresses, transport)
}

// NewVersionedClient returns the right elasticsearch client for the current Stack version, as an interface
func NewVersionedClient(apikey, user, pwd string, addresses []string, transport http.RoundTripper) (Client, error) {
	version := common.MustNewVersion(version.GetDefaultVersion())
	if version.IsMajor(8) {
		c, err := newV8Client(apikey, user, pwd, addresses, transport)
		return clientV8{c}, err
	}
	c, err := newV7Client(apikey, user, pwd, addresses, transport)
	return clientV7{c}, err
}

func newV7Client(apikey, user, pwd string, addresses []string, transport http.RoundTripper) (*esv7.Client, error) {
	return esv7.NewClient(esv7.Config{
		APIKey:    apikey,
		Username:  user,
		Password:  pwd,
		Addresses: addresses,
		Transport: transport,
	})
}

func newV8Client(apikey, user, pwd string, addresses []string, transport http.RoundTripper) (*esv8.Client, error) {
	return esv8.NewClient(esv8.Config{
		APIKey:    apikey,
		Username:  user,
		Password:  pwd,
		Addresses: addresses,
		Transport: transport,
	})
}

type JSONResponse struct {
	content io.ReadCloser
	err     error
}

func (r JSONResponse) DecodeTo(i interface{}) error {
	if r.err != nil {
		return r.err
	}
	defer r.content.Close()
	err := json.NewDecoder(r.content).Decode(&i)
	return err
}

// each header has the format "key: value"
func makeRequest(method, path string, body interface{}, headers ...string) (*http.Request, error) {
	header := http.Header{
		"Content-Type": []string{"application/json"},
		"Accept":       []string{"application/json"},
	}
	for _, h := range headers {
		kv := strings.Split(h, ":")
		if len(kv) == 2 {
			header[kv[0]] = strings.Split(kv[1], ",")
		}
	}
	bs, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, path, ioutil.NopCloser(bytes.NewReader(bs)))
	if err != nil {
		return nil, err
	}
	req.Header = header
	return req, nil
}

func parseResponse(resp *http.Response, err error) JSONResponse {
	if err != nil {
		return JSONResponse{nil, err}
	}
	body := resp.Body
	if resp.StatusCode >= http.StatusMultipleChoices {
		buf := new(bytes.Buffer)
		buf.ReadFrom(body)
		body.Close()
		return JSONResponse{nil, errors.New(buf.String())}
	}
	return JSONResponse{body, nil}
}
