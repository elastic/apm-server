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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/apm-server/beater/config"

	"github.com/stretchr/testify/assert"
)

func TestFleetFetch(t *testing.T) {
	var (
		hasAuth       bool
		apikey        = "supersecret"
		name          = "webapp"
		version       = "1.0.0"
		path          = "/my/path/to/bundle.js.map"
		wantRes       = "sourcemap response"
		c             = http.DefaultClient
		sourceMapPath = "/api/fleet/artifact"
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, sourceMapPath, r.URL.Path)
		auth := r.Header.Get("Authorization")
		hasAuth = auth == "ApiKey "+apikey
		// zlib compress
		wr := zlib.NewWriter(w)
		defer wr.Close()
		wr.Write([]byte(wantRes))
	}))
	defer ts.Close()

	fleetCfg := &config.Fleet{
		Hosts:        []string{ts.URL[7:]},
		Protocol:     "http",
		AccessAPIKey: apikey,
		TLS:          nil,
	}

	cfgs := []config.SourceMapMetadata{
		{
			ServiceName:    name,
			ServiceVersion: version,
			BundleFilepath: path,
			SourceMapURL:   sourceMapPath,
		},
	}
	fb, err := newFleetStore(c, fleetCfg, cfgs)
	assert.NoError(t, err)

	gotRes, err := fb.fetch(context.Background(), name, version, path)
	assert.NoError(t, err)

	assert.Equal(t, wantRes, gotRes)

	assert.True(t, hasAuth)
}
