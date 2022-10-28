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
	"context"
	"io"
	"net/http"
	"net/url"

	"go.elastic.co/apm/module/apmhttp/v2"

	"github.com/elastic/elastic-agent-libs/kibana"

	"github.com/elastic/apm-server/internal/version"
)

type ClientConfig = kibana.ClientConfig

// Client provides an interface for sending requests to Kibana.
type Client struct {
	client *kibana.Client
}

// NewClient returns a Client for making HTTP requests to Kibana.
func NewClient(cfg ClientConfig) (*Client, error) {
	// Never fetch the Kibana version; we don't use it.
	cfg.IgnoreVersion = true
	client, err := kibana.NewClientWithConfig(
		&cfg, "apm-server",
		version.Version,
		version.CommitHash(),
		version.CommitTime().String(),
	)
	if err != nil {
		return nil, err
	}
	client.HTTP = apmhttp.WrapClient(client.HTTP)
	return &Client{client: client}, nil
}

// Send sends an HTTP request to Kibana, and returns the unparsed response.
func (c *Client) Send(
	ctx context.Context, method, extraPath string,
	params url.Values,
	headers http.Header, body io.Reader,
) (*http.Response, error) {
	return c.client.SendWithContext(ctx, method, extraPath, params, headers, body)
}
