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
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/config"
	logs "github.com/elastic/apm-server/log"
)

type fleetStore struct {
	apikey    string
	c         *http.Client
	fleetURLs map[key]string
}

type key struct {
	ServiceName    string
	ServiceVersion string
	BundleFilepath string
}

// NewFleetStore returns an instance of Store for interacting with sourcemaps
// stored in Fleet-Server.
func NewFleetStore(
	c *http.Client,
	fleetCfg *config.Fleet,
	cfgs []config.SourceMapMetadata,
	expiration time.Duration,
) (*Store, error) {
	if len(fleetCfg.Hosts) < 1 {
		return nil, errors.New("no fleet hosts present for fleet store")
	}
	logger := logp.NewLogger(logs.Sourcemap)
	s, err := newFleetStore(c, fleetCfg, cfgs)
	if err != nil {
		return nil, err
	}
	return newStore(s, logger, expiration)
}

func newFleetStore(
	c *http.Client,
	fleetCfg *config.Fleet,
	cfgs []config.SourceMapMetadata,
) (fleetStore, error) {
	// TODO(stn): This is assuming we'll get a single fleet address right now.
	// If we get more, how do we know which artifact is where? Are they
	// present on all fleet-servers and we should rr through the addresses?
	host := fleetCfg.Hosts[0]
	fleetURLs := make(map[key]string)

	for _, cfg := range cfgs {
		k := key{cfg.ServiceName, cfg.ServiceVersion, cfg.BundleFilepath}
		u, err := common.MakeURL(fleetCfg.Protocol, cfg.SourceMapURL, host, 8220)
		if err != nil {
			return fleetStore{}, err
		}
		fleetURLs[k] = u
	}
	return fleetStore{
		apikey:    "ApiKey " + fleetCfg.AccessAPIKey,
		fleetURLs: fleetURLs,
		c:         c,
	}, nil
}

func (f fleetStore) fetch(ctx context.Context, name, version, path string) (string, error) {
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

	// Verify that we should only get 200 back from fleet-server
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failure querying fleet: statuscode=%d response=(failed to read body)", resp.StatusCode)
		}
		return "", fmt.Errorf("failure querying fleet: statuscode=%d response=%s", resp.StatusCode, body)
	}

	// Do the headers show compression???

	buf := new(bytes.Buffer)

	// TODO: Caue said that the response is b64 encoded.
	// body := base64.NewDecoder(base64.StdEncoding, resp.Body)
	// Looking at the index in elasticsearch, currently
	// - no encryption
	// - zlib compression
	r, err := zlib.NewReader(resp.Body)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(buf, r); err != nil {
		return "", err
	}

	return buf.String(), nil
}
