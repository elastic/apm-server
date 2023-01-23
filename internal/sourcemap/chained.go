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

	"github.com/go-sourcemap/sourcemap"
)

// ChainedFetcher is a Fetcher that attempts fetching from each Fetcher in sequence.
type ChainedFetcher []Fetcher

// Fetch fetches a source map from Kibana.
//
// Fetch calls Fetch on each Fetcher in the chain, in sequence, until one returns
// a non-nil Consumer and nil error. If no Fetch call succeeds, then the last error
// will be returned.
func (c ChainedFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	var lastErr error
	for _, f := range c {
		consumer, err := f.Fetch(ctx, name, version, path)
		if err != nil {
			lastErr = err
			continue
		}
		if consumer != nil {
			return consumer, nil
		}
	}
	return nil, lastErr
}
