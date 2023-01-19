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

func GetIdentifiers(name string, version string, bundleFilepath string) []Identifier {
	urlPath, err := url.Parse(bundleFilepath)
	if err != nil {
		// bundleFilepath is not an url
		// use full match
		return []Identifier{{
			name:    name,
			version: version,
			path:    bundleFilepath,
		}}
	}

	identifiers := make([]Identifier, 0, 2)

	urlPath.RawQuery = ""
	urlPath.Fragment = ""

	// first try to match the full url
	identifiers = append(identifiers, Identifier{
		name:    name,
		version: version,
		path:    urlPath.String(),
	})

	// then try to match the url path
	identifiers = append(identifiers, Identifier{
		name:    name,
		version: version,
		path:    urlPath.Path,
	})

	return identifiers
}
