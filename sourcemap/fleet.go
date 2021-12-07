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

package sourcemap

import (
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/go-sourcemap/sourcemap"
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/beater/config"
)

const defaultFleetPort = 8220

var errMsgFleetFailure = errMsgFailure + " fleet"

type fleetFetcher struct {
	apikey        string
	c             *http.Client
	sourceMapURLs map[key]string
	fleetBaseURLs []string
}

type key struct {
	ServiceName    string
	ServiceVersion string
	BundleFilepath string
}

// NewFleetFetcher returns a Fetcher which fetches source maps via Fleet Server.
func NewFleetFetcher(c *http.Client, fleetCfg *config.Fleet, cfgs []config.SourceMapMetadata) (Fetcher, error) {
	if len(fleetCfg.Hosts) < 1 {
		return nil, errors.New("no fleet hosts present for fleet store")
	}

	sourceMapURLs := make(map[key]string)
	for _, cfg := range cfgs {
		k := key{cfg.ServiceName, cfg.ServiceVersion, cfg.BundleFilepath}
		sourceMapURLs[k] = cfg.SourceMapURL
	}

	fleetBaseURLs := make([]string, len(fleetCfg.Hosts))
	for i, host := range fleetCfg.Hosts {
		baseURL, err := common.MakeURL(fleetCfg.Protocol, "", host, defaultFleetPort)
		if err != nil {
			return nil, err
		}
		fleetBaseURLs[i] = baseURL
	}

	return fleetFetcher{
		apikey:        "ApiKey " + fleetCfg.AccessAPIKey,
		fleetBaseURLs: fleetBaseURLs,
		sourceMapURLs: sourceMapURLs,
		c:             c,
	}, nil
}

// Fetch fetches a source map from Fleet Server.
func (f fleetFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	k := key{name, version, path}
	sourceMapURL, ok := f.sourceMapURLs[k]
	if !ok {
		return nil, fmt.Errorf("unable to find sourcemap.url for service.name=%s service.version=%s bundle.path=%s",
			name, version, path,
		)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		sourcemap string
		err       error
	}

	results := make(chan result)
	var wg sync.WaitGroup
	for _, baseURL := range f.fleetBaseURLs {
		wg.Add(1)
		go func(fleetURL string) {
			defer wg.Done()
			sourcemap, err := sendRequest(ctx, f, fleetURL)
			select {
			case <-ctx.Done():
			case results <- result{sourcemap, err}:
			}
		}(baseURL + sourceMapURL)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var err error
	for result := range results {
		err = result.err
		if err == nil {
			return parseSourceMap(result.sourcemap)
		}
	}

	if err != nil {
		return nil, err
	}
	// No results were received: context was cancelled.
	return nil, ctx.Err()
}

func sendRequest(ctx context.Context, f fleetFetcher, fleetURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fleetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", f.apikey)

	resp, err := f.c.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Verify that we should only get 200 back from fleet-server
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf(errMsgFleetFailure, ": statuscode=%d response=(failed to read body)", resp.StatusCode)
		}
		return "", fmt.Errorf(errMsgFleetFailure, ": statuscode=%d response=%s", resp.StatusCode, body)
	}

	// Looking at the index in elasticsearch, currently
	// - no encryption
	// - zlib compression
	r, err := zlib.NewReader(resp.Body)
	if err != nil {
		return "", err
	}

	var m map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return "", err
	}
	return string(m["sourceMap"]), nil
}
