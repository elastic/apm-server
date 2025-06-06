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

package kibana_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/integrationservertest/internal/kibana"
)

// getHttpClient instantiate a http.Client backed by a recorder.Recorder to be used in testing
// scenarios.
// To update any fixture, delete it from the filesystem and run the test again.
func getHttpClient(t *testing.T) (*recorder.Recorder, *http.Client) {
	t.Helper()

	var err error
	var rec *recorder.Recorder
	var hc *http.Client

	tr := http.DefaultTransport
	rec, err = recorder.NewAsMode(path.Join("testdata", "fixtures", t.Name()), recorder.ModeReplaying, tr)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := rec.Stop(); err != nil {
			t.Fatalf("cannot stop HTTP recorder: %s", err)
		}
	})

	// As we are replacing the url in saved data, we also need to use a
	// custom matcher to ignore the hostname.
	rec.SetMatcher(func(r *http.Request, i cassette.Request) bool {
		// copy to avoid changing request being handled
		u := *r.URL
		u.Host = "test"
		return r.Method == i.Method && u.String() == i.URL
	})

	// Filter out dynamic & sensitive data/headers before saving the fixture.
	// Tests will be using the real values when recording the interactions.
	rec.AddSaveFilter(func(i *cassette.Interaction) error {
		delete(i.Request.Headers, "Authorization")

		delete(i.Response.Headers, "Reporting-Endpoints")
		delete(i.Response.Headers, "Kbn-License-Sig")
		delete(i.Response.Headers, "X-Found-Handling-Cluster")

		var sanitizeBody = func(content string) string {
			newcontent := regexp.MustCompile(`"api_key":"[a-zA-Z0-9_\-\:]+"`).
				ReplaceAll([]byte(content), []byte(`"api_key":"REDACTED"`))
			newcontent = regexp.MustCompile(`"secret_token":{"type":"text","value":"[a-zA-Z0-9\-_\:]*"}`).
				ReplaceAll(newcontent, []byte(`"secret_token":{"type":"text","value":"REDACTED"}`))
			newcontent = regexp.MustCompile(`"secret_token":"[a-zA-Z0-9\-_\:]*"`).
				ReplaceAll(newcontent, []byte(`"secret_token":"REDACTED"`))
			newcontent = regexp.MustCompile(`"url":{(.*?)"value":"https:\/\/[a-z\-0-9\.]*\:[0-9]*"(.*?)}`).
				ReplaceAll(newcontent, []byte(`"url":{$1"value":"https://test"$2}`))
			return string(newcontent)
		}

		i.Request.Body = sanitizeBody(i.Request.Body)
		i.Response.Body = sanitizeBody(i.Response.Body)

		var redactURL = func(u string) string {
			ur, err := url.Parse(u)
			require.NoError(t, err)
			ur.Host = "test"
			return ur.String()
		}

		i.Request.URL = redactURL(i.Request.URL)

		return nil
	})

	hc = &http.Client{
		Transport: rec,
		Timeout:   10 * time.Second,
	}

	return rec, hc
}

func newRecordedTestClient(t *testing.T) *kibana.Client {
	kibanaURL := os.Getenv("KIBANA_URL")
	username := os.Getenv("KIBANA_USERNAME")
	password := os.Getenv("KIBANA_PASSWORD")
	kbc, err := kibana.NewClient(kibanaURL, username, password)
	require.NoError(t, err)
	_, httpc := getHttpClient(t)
	kbc.Client = *httpc

	return kbc
}

func TestClient_GetPackagePolicyByID(t *testing.T) {
	kbc := newRecordedTestClient(t)

	policy, err := kbc.GetPackagePolicyByID(context.Background(), "elastic-cloud-apm")
	require.NoError(t, err)
	assert.Equal(t, "Elastic APM", policy.Name)
}

func TestClient_UpdatePackagePolicyByID(t *testing.T) {
	kbc := newRecordedTestClient(t)

	ctx := context.Background()
	policyID := "elastic-cloud-apm"
	err := kbc.UpdatePackagePolicyByID(ctx, policyID,
		kibana.PackagePolicy{
			Name:        "Elastic APM",
			Description: "Hello World",
			Package: kibana.PackagePolicyPkg{
				Name:    "apm",
				Version: "9.1.0-SNAPSHOT",
			},
		},
	)
	require.NoError(t, err)

	policy, err := kbc.GetPackagePolicyByID(ctx, policyID)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", policy.Description)
}

// To regenerate the fixture, first delete the existing fixture. Then, you will
// need a deployed cluster that has been upgraded from 7.x to 8.x, and has
// ingested APM data.
//
// For reference, here is the minimal required code to run (in the main test folder).
// Note that the t.Fail() is intended in order to stop the test cleanup.
//
//	func TestResolveMigrationDeprecationsSetup(t *testing.T) {
//		expect := dataStreamsExpectations(asserts.DataStreamExpectation{
//			PreferIlm:        true,
//			DSManagedBy:      managedByILM,
//			IndicesManagedBy: []string{managedByILM},
//		})
//
//		runner := testStepsRunner{
//			Steps: []testStep{
//				createStep{
//					DeployVersion:     vsCache.GetLatestSnapshot(t, "7.17").Version,
//					APMDeploymentMode: apmStandalone,
//				},
//				ingestV7Step{},
//				upgradeV7Step{NewVersion: vsCache.GetLatestSnapshot(t, "8").Version},
//				ingestStep{CheckDataStreams: expect},
//			},
//		}
//		runner.Run(t)
//		t.Fail()
//	}
//
// Once you have the cluster setup, set the appropriate environment variables
// (KIBANA_URL, KIBANA_USERNAME and KIBANA_PASSWORD) and run this test.
func TestClient_ResolveMigrationDeprecations(t *testing.T) {
	kbc := newRecordedTestClient(t)

	ctx := context.Background()

	// Check that system indices are not migrated.
	status, err := kbc.QuerySystemIndicesMigrationStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, kibana.MigrationNeeded, status)

	// Check that there are some critical deprecation warnings.
	deprecations, err := kbc.QueryCriticalESDeprecations(ctx)
	require.NoError(t, err)
	require.Greater(t, len(deprecations), 0)
	for _, deprecation := range deprecations {
		require.True(t, deprecation.IsCritical)
	}

	// Resolve them.
	// NOTE: This test will not be effective against different deprecation
	// types than what was initially set.
	err = kbc.ResolveMigrationDeprecations(ctx)
	require.NoError(t, err)

	// Check that system indices are migrated.
	status, err = kbc.QuerySystemIndicesMigrationStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, kibana.NoMigrationNeeded, status)

	// Check that there are no more warnings.
	deprecations, err = kbc.QueryCriticalESDeprecations(ctx)
	require.NoError(t, err)
	assert.Len(t, deprecations, 0)
}
