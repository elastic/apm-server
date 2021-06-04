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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/elastic/apm-server/beater/config"
)

// FleetStore handles making sourcemap requests to a Fleet-Server.
type FleetStore struct {
	apikey    string
	c         *http.Client
	fleetURLs map[key]string
}

type key struct {
	ServiceName    string
	ServiceVersion string
	BundleFilepath string
}

// NewFleetStore returns an instance of ESStore for interacting with sourcemaps
// stored in Fleet-Server.
func NewFleetStore(apikey string, cfgs []config.SourceMapConfig, c *http.Client) FleetStore {
	fleetURLs := make(map[key]string)
	for _, cfg := range cfgs {
		k := key{cfg.ServiceName, cfg.ServiceVersion, cfg.BundleFilepath}
		fleetURLs[k] = cfg.SourceMapURL
	}
	return FleetStore{
		apikey:    "ApiKey " + apikey,
		fleetURLs: fleetURLs,
		c:         c,
	}
}

func (f FleetStore) fetch(ctx context.Context, name, version, path string) (string, error) {
	k := key{name, version, path}
	fleetURL, ok := f.fleetURLs[k]
	if !ok {
		return "", fmt.Errorf("unable to find sourcemap.url for service.name=%s service.version=%s bundle.path=%s",
			name, version, path,
		)
	}
	req, err := http.NewRequest(http.MethodGet, fleetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", f.apikey)

	resp, err := f.c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)

	io.Copy(buf, resp.Body)

	return buf.String(), nil
}
