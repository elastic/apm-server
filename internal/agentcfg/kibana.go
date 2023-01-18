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

package agentcfg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/kibana"
)

// Error Messages used to signal fetching errors
const (
	ErrMsgReadKibanaResponse = "unable to read Kibana response body"
	ErrMsgSendToKibanaFailed = "sending request to kibana failed"
	ErrUnauthorized          = "Unauthorized"
)

const endpoint = "/api/apm/settings/agent-configuration/search"

// KibanaFetcher holds static information and information shared between requests.
// It implements the Fetch method to retrieve agent configuration information.
type KibanaFetcher struct {
	*cache
	logger *logp.Logger
	client *kibana.Client
}

// NewKibanaFetcher returns a KibanaFetcher instance.
//
// NewKibanaFetcher will return an error if passed a nil client.
func NewKibanaFetcher(client *kibana.Client, cacheExpiration time.Duration) (*KibanaFetcher, error) {
	if client == nil {
		return nil, errors.New("client is required")
	}
	logger := logp.NewLogger("agentcfg")
	return &KibanaFetcher{
		client: client,
		logger: logger,
		cache:  newCache(logger, cacheExpiration),
	}, nil
}

// Fetch retrieves agent configuration, fetched from Kibana or a local temporary cache.
func (f *KibanaFetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	req := func() (Result, error) {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(query); err != nil {
			return Result{}, err
		}
		return newResult(f.request(ctx, &buf))
	}
	return f.fetch(query, req)
}

func (f *KibanaFetcher) request(ctx context.Context, r io.Reader) ([]byte, error) {
	resp, err := f.client.Send(ctx, http.MethodPost, endpoint, nil, nil, r)
	if err != nil {
		return nil, errors.Wrap(err, ErrMsgSendToKibanaFailed)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	result, err := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, errors.Errorf("agentcfg kibana request failed with status code %d: %s", resp.StatusCode, string(result))
	}
	if err != nil {
		return nil, errors.Wrap(err, ErrMsgReadKibanaResponse)
	}
	return result, nil
}
