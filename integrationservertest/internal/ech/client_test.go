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

package ech_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

// recordedHTTPClient instantiates a http.Client backed by a recorder.Recorder to be
// used in testing scenarios.
// To update any fixture, delete it from the filesystem and run the test again.
func recordedHTTPClient(t *testing.T) (*recorder.Recorder, *http.Client) {
	fixturesFilename := path.Join("testdata", "fixtures", t.Name())
	rec, err := recorder.NewAsMode(fixturesFilename, recorder.ModeReplaying, http.DefaultTransport)
	require.NoError(t, err)

	t.Cleanup(func() {
		if stopErr := rec.Stop(); stopErr != nil {
			t.Fatalf("cannot stop HTTP recorder: %s", stopErr)
		}
	})

	// Filter out dynamic & sensitive data/headers before saving the fixture.
	// Tests will be using the real values when recording the interactions.
	rec.AddSaveFilter(func(i *cassette.Interaction) error {
		redactURL := func(u string) string {
			ur, err := url.Parse(u)
			require.NoError(t, err)
			ur.Host = "test"
			return ur.String()
		}

		i.Request.URL = redactURL(i.Request.URL)
		delete(i.Request.Headers, "Authorization")

		delete(i.Response.Headers, "X-Elastic-Api-Key-Expiration")

		return nil
	})

	// As we are replacing the url in saved data, we also need to use a
	// custom matcher to ignore the hostname.
	rec.SetMatcher(func(r *http.Request, i cassette.Request) bool {
		// Copy to avoid changing request being handled
		u := *r.URL
		u.Host = "test"
		return r.Method == i.Method && u.String() == i.URL
	})

	httpClient := &http.Client{
		Transport: rec,
		Timeout:   10 * time.Second,
	}

	return rec, httpClient
}

func newRecordedClient(t *testing.T) *ech.Client {
	endpoint := os.Getenv("EC_URL")
	apiKey := os.Getenv("EC_API_KEY")
	_, httpClient := recordedHTTPClient(t)
	ecc, err := ech.NewClient(endpoint, apiKey, ech.WithHTTPClient(httpClient))
	require.NoError(t, err)
	return ecc
}

func TestClient_GetVersions(t *testing.T) {
	ecc := newRecordedClient(t)
	region := os.Getenv("EC_REGION")

	versions, err := ecc.GetVersions(context.Background(), region)
	require.NoError(t, err)

	allNoSuffix := func() bool {
		for _, v := range versions {
			if v.Version.Suffix != "" {
				return false
			}
		}
		return true
	}

	assert.True(t, len(versions) > 0)
	assert.True(t, allNoSuffix())
}

func TestClient_GetSnapshotVersions(t *testing.T) {
	ecc := newRecordedClient(t)
	region := os.Getenv("EC_REGION")

	versions, err := ecc.GetSnapshotVersions(context.Background(), region)
	require.NoError(t, err)

	allSnapshots := func() bool {
		for _, v := range versions {
			if !v.Version.IsSnapshot() {
				return false
			}
		}
		return true
	}

	assert.True(t, len(versions) > 0)
	assert.True(t, allSnapshots())
}
