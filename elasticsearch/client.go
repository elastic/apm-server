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
	"io"
	"net/http"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/version"

	v7 "github.com/elastic/go-elasticsearch/v7"
	v7esapi "github.com/elastic/go-elasticsearch/v7/esapi"

	v8 "github.com/elastic/go-elasticsearch/v8"
	v8esapi "github.com/elastic/go-elasticsearch/v8/esapi"
)

// Client is an interface designed to abstract away version differences between elasticsearch clients
type Client interface {
	// Search performs a query against the given index with the given body
	Search(index string, body io.Reader) (int, io.ReadCloser, error)
	SecurityHasPrivilegesRequest(body io.Reader, header http.Header) (int, io.ReadCloser, error)
}

type clientV8 struct {
	client *v8.Client
}

// Search satisfies the Client interface for version 8
func (c clientV8) Search(index string, body io.Reader) (int, io.ReadCloser, error) {
	return v8Response(c.client.Search(
		c.client.Search.WithContext(context.Background()),
		c.client.Search.WithIndex(index),
		c.client.Search.WithBody(body),
		c.client.Search.WithTrackTotalHits(true),
		c.client.Search.WithPretty(),
	))
}

func (c clientV8) SecurityHasPrivilegesRequest(body io.Reader, header http.Header) (int, io.ReadCloser, error) {
	hasPrivileges := v8esapi.SecurityHasPrivilegesRequest{Body: body, Header: header}
	return v8Response(hasPrivileges.Do(context.Background(), c.client))
}

func v8Response(response *v8esapi.Response, err error) (int, io.ReadCloser, error) {
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}

type clientV7 struct {
	client *v7.Client
}

// Search satisfies the Client interface for version 7
func (c clientV7) Search(index string, body io.Reader) (int, io.ReadCloser, error) {
	return v7Response(c.client.Search(
		c.client.Search.WithContext(context.Background()),
		c.client.Search.WithIndex(index),
		c.client.Search.WithBody(body),
		c.client.Search.WithTrackTotalHits(true),
		c.client.Search.WithPretty(),
	))
}

func (c clientV7) SecurityHasPrivilegesRequest(body io.Reader, header http.Header) (int, io.ReadCloser, error) {
	hasPrivileges := v7esapi.SecurityHasPrivilegesRequest{Body: body, Header: header}
	return v7Response(hasPrivileges.Do(context.Background(), c.client))
}

func v7Response(response *v7esapi.Response, err error) (int, io.ReadCloser, error) {
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
