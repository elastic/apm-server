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
	"io"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/go-sourcemap/sourcemap"
	"github.com/pkg/errors"
)

var errMsgFleetFailure = errMsgFailure + " fleet"

type fleetFetcher struct {
	authorization     string
	httpClient        *http.Client
	fleetServerURLs   []*url.URL
	sourceMapURLPaths map[key]string
}

type key struct {
	ServiceName    string
	ServiceVersion string
	BundleFilepath string
}

// FleetArtifactReference holds information mapping a source map to a
// Fleet Artifact URL path. The server uses this to fetch the source
// map content via Fleet Server.
type FleetArtifactReference struct {
	// ServiceName holds the service name to which the source map relates.
	ServiceName string

	// ServiceVersion holds the service version to which the source map relates.
	ServiceVersion string

	// BundleFilepath holds the bundle file path to which the source map relates.
	BundleFilepath string

	// FleetServerURLPath holds the URL path for fetching the source map,
	// by appending it to each of the Fleet Server URLs.
	FleetServerURLPath string
}

// NewFleetFetcher returns a Fetcher which fetches source maps via Fleet Server.
func NewFleetFetcher(
	httpClient *http.Client, apiKey string,
	fleetServerURLs []*url.URL,
	refs []FleetArtifactReference,
) (Fetcher, error) {

	if len(fleetServerURLs) == 0 {
		return nil, errors.New("no fleet-server hosts present for fleet store")
	}

	sourceMapURLPaths := make(map[key]string)
	for _, ref := range refs {
		k := key{ref.ServiceName, ref.ServiceVersion, ref.BundleFilepath}
		sourceMapURLPaths[k] = ref.FleetServerURLPath
	}

	return fleetFetcher{
		authorization:     "ApiKey " + apiKey,
		httpClient:        httpClient,
		fleetServerURLs:   fleetServerURLs,
		sourceMapURLPaths: sourceMapURLPaths,
	}, nil
}

// Fetch fetches a source map from Fleet Server.
func (f fleetFetcher) Fetch(ctx context.Context, name, version, bundleFilepath string) (*sourcemap.Consumer, error) {
	sourceMapURLPath, ok := f.sourceMapURLPaths[key{name, version, maybeParseURLPath(bundleFilepath)}]
	if !ok {
		return nil, fmt.Errorf("unable to find sourcemap.url for service.name=%s service.version=%s bundle.path=%s",
			name, version, bundleFilepath,
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
	for _, baseURL := range f.fleetServerURLs {
		// TODO(axw) use URL.JoinPath when we upgrade to Go 1.19.
		u := *baseURL
		u.Path = path.Join(u.Path, sourceMapURLPath)
		artifactURL := u.String()

		wg.Add(1)
		go func() {
			defer wg.Done()
			sourcemap, err := sendRequest(ctx, f, artifactURL)
			select {
			case <-ctx.Done():
			case results <- result{sourcemap, err}:
			}
		}()
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fleetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", f.authorization)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Verify that we should only get 200 back from fleet-server
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
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
