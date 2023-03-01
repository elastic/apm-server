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
	"context"
	"fmt"
	"net/url"

	"github.com/go-sourcemap/sourcemap"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

type SourcemapFetcher struct {
	metadata MetadataFetcher
	backend  Fetcher
	logger   *logp.Logger
}

func NewSourcemapFetcher(metadata MetadataFetcher, backend Fetcher) *SourcemapFetcher {
	s := &SourcemapFetcher{
		metadata: metadata,
		backend:  backend,
		logger:   logp.NewLogger(logs.Sourcemap),
	}

	return s
}

func (s *SourcemapFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	original := identifier{name: name, version: version, path: path}

	select {
	case <-s.metadata.ready():
		if err := s.metadata.err(); err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("error waiting for metadata fetcher to be ready: %w", ctx.Err())
	default:
		return nil, fmt.Errorf("metadata fetcher is not ready: %w", errFetcherUnvailable)
	}

	if i, ok := s.metadata.getID(original); ok {
		// Only fetch from ES if the sourcemap id exists
		return s.fetch(ctx, i)
	}

	if urlPath, err := url.Parse(path); err == nil {
		// The sourcemap coule be stored in ES with a relative
		// bundle filepath but the request came in with an
		// absolute path
		original.path = urlPath.Path
		if urlPath.Path != path {
			// The sourcemap could be stored on ES under a certain host
			// but a request came in from a different host.
			// Look for an alias to the url path to retrieve the correct
			// host and fetch the sourcemap
			if i, ok := s.metadata.getID(original); ok {
				return s.fetch(ctx, i)
			}
		}

		// Clean the url and try again if the result is different from
		// the original bundle filepath
		urlPath.RawQuery = ""
		urlPath.Fragment = ""
		urlPath = urlPath.JoinPath()
		cleanPath := urlPath.String()

		if cleanPath != path {
			s.logger.Debugf("original filepath %s converted to %s", path, cleanPath)
			return s.Fetch(ctx, name, version, cleanPath)
		}
	}

	return nil, fmt.Errorf("unable to find sourcemap.url for service.name=%s service.version=%s bundle.path=%s", name, version, path)
}

func (s *SourcemapFetcher) fetch(ctx context.Context, key *identifier) (*sourcemap.Consumer, error) {
	c, err := s.backend.Fetch(ctx, key.name, key.version, key.path)

	// log a message if the sourcemap is present in the cache but the backend fetcher did not
	// find it.
	if err == nil && c == nil {
		return nil, fmt.Errorf("unable to find sourcemap for service.name=%s service.version=%s bundle.path=%s", key.name, key.version, key.path)
	}

	return c, err
}
