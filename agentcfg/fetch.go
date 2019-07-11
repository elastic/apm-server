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
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
)

// Error Messages used to signal fetching errors
const (
	ErrMsgSendToKibanaFailed = "sending request to kibana failed"
	ErrMsgMultipleChoices    = "multiple configurations found"
	ErrMsgReadKibanaResponse = "unable to read Kibana response body"
)
const endpoint = "/api/apm/settings/agent-configuration/search"

// Fetcher holds static information and information shared between requests.
// It implements the Fetch method to retrieve agent configuration information.
type Fetcher struct {
	docCache *cache
	logger   *logp.Logger
	kbClient kibana.Client
}

// NewFetcher returns a Fetcher instance.
func NewFetcher(kbClient kibana.Client, cacheExp time.Duration) *Fetcher {
	logger := logp.NewLogger("agentcfg")
	return &Fetcher{
		kbClient: kbClient,
		logger:   logger,
		docCache: newCache(logger, cacheExp),
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

	resp, err := f.kbClient.Send(http.MethodPost, endpoint, nil, nil, r)
	if err != nil {
		return nil, errors.Wrap(err, ErrMsgSendToKibanaFailed)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	result, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusMultipleChoices {
		return nil, errors.Wrap(errors.New(string(result)), ErrMsgMultipleChoices)
	} else if resp.StatusCode > http.StatusMultipleChoices {
		return nil, errors.New(string(result))
	}
	if err != nil {
		return nil, errors.Wrap(err, ErrMsgReadKibanaResponse)
	}
	return result, nil
}
