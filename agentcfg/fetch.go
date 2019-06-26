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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/kibana"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/convert"
)

const (
	endpoint = "/api/apm/settings/cm/search"
)

// Fetcher holds static information and information shared between requests.
// It implements the Fetch method to retrieve agent configuration information.
type Fetcher struct {
	kbClient   *kibana.Client
	docCache   *cache
	logger     *logp.Logger
	minVersion common.Version
}

// NewFetcher returns a Fetcher instance.
func NewFetcher(kbClient *kibana.Client, cacheExp time.Duration) *Fetcher {
	logger := logp.NewLogger("agentcfg")
	return &Fetcher{
		kbClient:   kbClient,
		logger:     logger,
		docCache:   newCache(logger, cacheExp),
		minVersion: common.Version{Major: 7, Minor: 3},
	}
}

// Fetch retrieves agent configuration, fetched from Kibana or a local temporary cache.
func (f *Fetcher) Fetch(q Query, err error) (map[string]string, string, error) {
	req := func(query Query) (*Doc, error) {
		var doc Doc
		resultBytes, err := f.request(convert.ToReader(query), err)
		err = convert.FromBytes(resultBytes, &doc, err)
		return &doc, err
	}

	doc, err := f.docCache.fetchAndAdd(q, req)
	if err != nil {
		return nil, "", err
	}

	return doc.Source.Settings, doc.ID, nil
}

func (f *Fetcher) request(r io.Reader, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if f.kbClient == nil {
		return nil, errors.New("no configured Kibana Client: provide apm-server.kibana.* settings")
	}
	if version := f.kbClient.GetVersion(); version.LessThan(&f.minVersion) {
		return nil, fmt.Errorf("needs Kibana version %s or higher", f.minVersion.String())
	}
	resp, err := f.kbClient.Send(http.MethodPost, endpoint, nil, nil, r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New(string(result))
	}
	return result, err
}
