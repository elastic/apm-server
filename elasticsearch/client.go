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
	"io/ioutil"
	"net/http"
	"strings"

	"go.elastic.co/apm/module/apmelasticsearch"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/version"
	esv7 "github.com/elastic/go-elasticsearch/v7"
	esapiv7 "github.com/elastic/go-elasticsearch/v7/esapi"
	esutilv7 "github.com/elastic/go-elasticsearch/v7/esutil"
	esv8 "github.com/elastic/go-elasticsearch/v8"
	esutilv8 "github.com/elastic/go-elasticsearch/v8/esutil"
)

var retryableStatuses = []int{
	http.StatusTooManyRequests,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}

// Client is an interface designed to abstract away version differences between elasticsearch clients
type Client interface {
	// NewBulkIndexer returns a new BulkIndexer using this client for making the requests.
	NewBulkIndexer(BulkIndexerConfig) (BulkIndexer, error)

	// Perform satisfies esapi.Transport
	Perform(*http.Request) (*http.Response, error)
}

type clientV8 struct {
	*esv8.Client
}

func (c clientV8) NewBulkIndexer(config BulkIndexerConfig) (BulkIndexer, error) {
	indexer, err := esutilv8.NewBulkIndexer(esutilv8.BulkIndexerConfig{
		Client:        c.Client,
		NumWorkers:    config.NumWorkers,
		FlushBytes:    config.FlushBytes,
		FlushInterval: config.FlushInterval,
		OnError:       config.OnError,
		OnFlushStart:  config.OnFlushStart,
		OnFlushEnd:    config.OnFlushEnd,
		Index:         config.Index,
		Pipeline:      config.Pipeline,
		Timeout:       config.Timeout,
	})
	if err != nil {
		return nil, err
	}
	return v8BulkIndexer{indexer}, nil
}

type clientV7 struct {
	*esv7.Client
}

func (c clientV7) NewBulkIndexer(config BulkIndexerConfig) (BulkIndexer, error) {
	indexer, err := esutilv7.NewBulkIndexer(esutilv7.BulkIndexerConfig{
		Client:        c.Client,
		NumWorkers:    config.NumWorkers,
		FlushBytes:    config.FlushBytes,
		FlushInterval: config.FlushInterval,
		OnError:       config.OnError,
		OnFlushStart:  config.OnFlushStart,
		OnFlushEnd:    config.OnFlushEnd,
		Index:         config.Index,
		Pipeline:      config.Pipeline,
		Timeout:       config.Timeout,
	})
	if err != nil {
		return nil, err
	}
	return v7BulkIndexer{indexer}, nil
}

// ClientParams holds parameters for NewClientParams.
type ClientParams struct {
	// Config holds the user-defined configuration: Elasticsearch hosts,
	// max retries, etc.
	Config *Config

	// Transport holds a net/http.RoundTripper to use for sending requests
	// to Elasticsearch.
	//
	// If Transport is nil, then a net/http.Transport will be constructed
	// with NewHTTPTransport(Config).
	Transport http.RoundTripper
}

// NewClient returns a stack version-aware Elasticsearch client,
// equivalent to NewClientParams(ClientParams{Config: config}).
func NewClient(config *Config) (Client, error) {
	return NewClientParams(ClientParams{Config: config})
}

// NewClientParams returns a stack version-aware Elasticsearch client.
func NewClientParams(args ClientParams) (Client, error) {
	if args.Config == nil {
		return nil, errConfigMissing
	}

	transport := args.Transport
	if transport == nil {
		httpTransport, err := NewHTTPTransport(args.Config)
		if err != nil {
			return nil, err
		}
		transport = httpTransport
	}

	addrs, err := addresses(args.Config)
	if err != nil {
		return nil, err
	}

	var headers http.Header
	if len(args.Config.Headers) > 0 {
		headers = make(http.Header, len(args.Config.Headers))
		for k, v := range args.Config.Headers {
			headers.Set(k, v)
		}
	}

	var apikey string
	if args.Config.APIKey != "" {
		apikey = base64.StdEncoding.EncodeToString([]byte(args.Config.APIKey))
	}

	newClient := newV7Client
	if version := common.MustNewVersion(version.GetDefaultVersion()); version.IsMajor(8) {
		newClient = newV8Client
	}
	return newClient(
		apikey, args.Config.Username, args.Config.Password,
		addrs,
		headers,
		apmelasticsearch.WrapRoundTripper(transport),
		args.Config.MaxRetries,
		exponentialBackoff(args.Config.Backoff),
	)
}

func newV7Client(
	apikey, user, pwd string,
	addresses []string,
	headers http.Header,
	transport http.RoundTripper,
	maxRetries int,
	fn backoffFunc,
) (Client, error) {

	// Set the compatibility configuration if compatibility headers are configured.
	// Delete headers, as ES client takes care of setting them appropriately.
	var enableCompatibilityMode bool
	acceptHeader := headers.Get("Accept")
	contentTypeHeader := headers.Get("Content-Type")
	if strings.Contains(acceptHeader, "compatible-with=7") || strings.Contains(contentTypeHeader, "compatible-with=7") {
		enableCompatibilityMode = true
		headers.Del("Accept")
		headers.Del(("Content-Type"))
	}

	c, err := esv7.NewClient(esv7.Config{
		APIKey:                  apikey,
		Username:                user,
		Password:                pwd,
		Addresses:               addresses,
		Transport:               transport,
		Header:                  headers,
		RetryOnStatus:           retryableStatuses,
		EnableRetryOnTimeout:    true,
		RetryBackoff:            fn,
		EnableCompatibilityMode: enableCompatibilityMode,
		MaxRetries:              maxRetries,
	})
	if err != nil {
		return nil, err
	}
	return clientV7{c}, nil
}

func newV8Client(
	apikey, user, pwd string,
	addresses []string,
	headers http.Header,
	transport http.RoundTripper,
	maxRetries int,
	fn backoffFunc,
) (Client, error) {
	c, err := esv8.NewClient(esv8.Config{
		APIKey:               apikey,
		Username:             user,
		Password:             pwd,
		Addresses:            addresses,
		Transport:            transport,
		Header:               headers,
		RetryOnStatus:        retryableStatuses,
		EnableRetryOnTimeout: true,
		RetryBackoff:         fn,
		MaxRetries:           maxRetries,
	})
	if err != nil {
		return nil, err
	}
	return clientV8{c}, nil
}

func doRequest(ctx context.Context, transport esapiv7.Transport, req esapiv7.Request, out interface{}) error {
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
