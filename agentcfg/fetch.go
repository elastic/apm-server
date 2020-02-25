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
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/elastic/apm-server/utility"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
)

// Error Messages used to signal fetching errors
const (
	ErrMsgSendToKibanaFailed = "sending request to kibana failed"
	ErrMsgReadKibanaResponse = "unable to read Kibana response body"
)
const endpoint = "/api/apm/settings/agent-configuration/search"

// Fetcher holds static information and information shared between requests.
// It implements the Fetch method to retrieve agent configuration information.
type Fetcher struct {
	*cache
	logger *logp.Logger
	client kibana.Client
}

// NewFetcher returns a Fetcher instance.
func NewFetcher(client kibana.Client, cacheExpiration time.Duration) *Fetcher {
	logger := logp.NewLogger("agentcfg")
	return &Fetcher{
		client: client,
		logger: logger,
		cache:  newCache(logger, cacheExpiration),
	}
}

// Fetch retrieves agent configuration, fetched from Kibana or a local temporary cache.
func (f *Fetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	req := func() (Result, error) {
		return newResult(f.request(ctx, convert.ToReader(query)))
	}
	result, err := f.fetch(query, req)
	return sanitize(query.IsRum, result), err
}

func (f *Fetcher) request(ctx context.Context, r io.Reader) ([]byte, error) {
	resp, err := f.client.Send(ctx, http.MethodPost, endpoint, nil, nil, r)
	if err != nil {
		return nil, errors.Wrap(err, ErrMsgSendToKibanaFailed)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	result, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, errors.New(string(result))
	}
	if err != nil {
		return nil, errors.Wrap(err, ErrMsgReadKibanaResponse)
	}
	return result, nil
}

func sanitize(isRum bool, result Result) Result {
	if !isRum {
		return result
	}
	hasRumData := utility.Contains(result.Source.Agent, RumAgent) || result.Source.Agent == ""
	if !hasRumData {
		return zeroResult()
	}
	settings := Settings{}
	for k, v := range result.Source.Settings {
		if utility.Contains(k, RumSettings) {
			settings[k] = v
		}
	}
	return Result{Source: Source{Etag: result.Source.Etag, Settings: settings}}
}
