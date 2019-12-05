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
	"net/url"
	"strings"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/version"

	v7 "github.com/elastic/go-elasticsearch/v7"

	v8 "github.com/elastic/go-elasticsearch/v8"
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
	c *v8.Client
}

// Search satisfies the Client interface for version 8
func (v8 clientV8) Search(index string, body io.Reader) (int, io.ReadCloser, error) {
	response, err := v8.c.Search(
		v8.c.Search.WithContext(context.Background()),
		v8.c.Search.WithIndex(index),
		v8.c.Search.WithBody(body),
		v8.c.Search.WithTrackTotalHits(true),
		v8.c.Search.WithPretty(),
	)
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}

func (v8 clientV8) JSONRequest(method, path string, body interface{}, headers ...string) JSONResponse {
	req, err := makeRequest(method, path, body, headers...)
	if err != nil {
		return JSONResponse{nil, err}
	}
	return parseResponse(v8.c.Perform(req))
}

type clientV7 struct {
	c *v7.Client
}

// Search satisfies the Client interface for version 7
func (v7 clientV7) Search(index string, body io.Reader) (int, io.ReadCloser, error) {
	response, err := v7.c.Search(
		v7.c.Search.WithContext(context.Background()),
		v7.c.Search.WithIndex(index),
		v7.c.Search.WithBody(body),
		v7.c.Search.WithTrackTotalHits(true),
		v7.c.Search.WithPretty(),
	)
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}

func (v7 clientV7) JSONRequest(method, path string, body interface{}, headers ...string) JSONResponse {
	req, err := makeRequest(method, path, body, headers...)
	if err != nil {
		return JSONResponse{nil, err}
	}
	return parseResponse(v7.c.Perform(req))
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

func newV7Client(apikey, user, pwd string, addresses []string, transport http.RoundTripper) (*v7.Client, error) {
	return v7.NewClient(v7.Config{
		APIKey:    apikey,
		Username:  user,
		Password:  pwd,
		Addresses: addresses,
		Transport: transport,
	})
}

func newV8Client(apikey, user, pwd string, addresses []string, transport http.RoundTripper) (*v8.Client, error) {
	return v8.NewClient(v8.Config{
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
	bs, err := ioutil.ReadAll(r.content)
	if err != nil {
		return err
	}
	err = json.Unmarshal(bs, i)
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
	u, _ := url.Parse(path)
	req := &http.Request{
		Method:     method,
		URL:        u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
	}
	bs, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Body = ioutil.NopCloser(bytes.NewReader(bs))
		req.ContentLength = int64(len(bs))
	}
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
		return JSONResponse{nil, errors.New(buf.String())}
	}
	return JSONResponse{body, nil}
}
