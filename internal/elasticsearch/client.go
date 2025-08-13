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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.elastic.co/apm/module/apmelasticsearch/v2"

	"github.com/elastic/apm-server/internal/version"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

var retryableStatuses = []int{
	http.StatusTooManyRequests,
}

// apm-server doesn't use go-elasticsearch but we kept the user agent string for
// compatibility
var userAgent = fmt.Sprintf("Elastic-APM-Server/%s go-elasticsearch/%s", version.VersionWithQualifier(), version.Version)

type Client = elastictransport.Client

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

	// RetryOnError indicates which client errors should be retried.
	// Optional.
	RetryOnError func(*http.Request, error) bool

	// Logger holds a logger
	Logger *logp.Logger
}

// NewClient returns a stack version-aware Elasticsearch client,
// equivalent to NewClientParams(ClientParams{Config: config}).
func NewClient(config *Config, logger *logp.Logger) (*Client, error) {
	return NewClientParams(ClientParams{Config: config, Logger: logger})
}

// NewClientParams returns a stack version-aware Elasticsearch client.
func NewClientParams(args ClientParams) (*Client, error) {
	if args.Config == nil {
		return nil, errConfigMissing
	}

	transport := args.Transport
	if transport == nil {
		if args.Logger == nil {
			args.Logger = logp.NewNopLogger()
		}

		httpTransport, err := NewHTTPTransport(args.Config, args.Logger)
		if err != nil {
			return nil, err
		}
		transport = httpTransport
	}

	addrs, err := addresses(args.Config)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header, len(args.Config.Headers)+2)
	if len(args.Config.Headers) > 0 {
		for k, v := range args.Config.Headers {
			headers.Set(k, v)
		}
	}

	headers.Set("X-Elastic-Product-Origin", "observability")
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", userAgent)
	}

	var apikey string
	if args.Config.APIKey != "" {
		apikey = base64.StdEncoding.EncodeToString([]byte(args.Config.APIKey))
	}

	return elastictransport.New(elastictransport.Config{
		APIKey:        apikey,
		Username:      args.Config.Username,
		Password:      args.Config.Password,
		URLs:          addrs,
		Header:        headers,
		Transport:     apmelasticsearch.WrapRoundTripper(transport),
		MaxRetries:    args.Config.MaxRetries,
		RetryBackoff:  exponentialBackoff(args.Config.Backoff),
		RetryOnError:  args.RetryOnError,
		RetryOnStatus: retryableStatuses,
	})
}

func doRequest(client *elastictransport.Client, req *http.Request, out interface{}) error {
	resp, err := client.Perform(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		bytes, err := io.ReadAll(resp.Body)
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
