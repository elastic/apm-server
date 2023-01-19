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
	"net/url"

	"github.com/go-sourcemap/sourcemap"
)

// Fetcher is an interface for fetching a source map with a given service name, service version,
// and bundle filepath.
type Fetcher interface {
	// Fetch fetches a source map with a given service name, service version, and bundle filepath.
	//
	// If there is no such source map available, Fetch returns a nil Consumer.
	Fetch(ctx context.Context, name string, version string, bundleFilepath string) (*sourcemap.Consumer, error)
}

type Identifier struct {
	name    string
	version string
	path    string
}

type Metadata struct {
	id          Identifier
	contentHash string
}

func GetAliases(name string, version string, bundleFilepath string) []Identifier {
	urlPath, err := url.Parse(bundleFilepath)
	if err != nil {
		// bundleFilepath is not an url so it
		// has no alias.
		// use full match
		return nil
	}

	if urlPath.String() == urlPath.Path {
		// "/foo.bundle.js.map" is a valid url
		// bundleFilepath is an url path
		// no alias
		return nil
	}

	urlPath.RawQuery = ""
	urlPath.Fragment = ""

	if urlPath.String() == bundleFilepath {
		// bundleFilepath is a valid url and it is
		// already clean.
		// Only return the url path as an alias
		return []Identifier{
			{
				name:    name,
				version: version,
				path:    urlPath.Path,
			},
		}
	}

	return []Identifier{
		// first try to match the full url
		{
			name:    name,
			version: version,
			path:    urlPath.String(),
		},

		// then try to match the url path
		{
			name:    name,
			version: version,
			path:    urlPath.Path,
		},
	}
}
