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

package systemtest

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestMain(m *testing.M) {
	log.Println("INFO: starting stack containers...")
	initContainers()
	if err := StartStackContainers(); err != nil {
		log.Fatalf("failed to start stack containers: %v", err)
	}
	initElasticSearch()
	if err := GeoIpLazyDownload(); err != nil {
		log.Fatalf("failed to download geoip database: %v", err)
	}
	initKibana()
	initSettings()
	initOTEL()
	log.Println("INFO: running system tests...")
	os.Exit(m.Run())
}

const requestBody = `{"metadata": { "service": {"name": "alice", "agent": {"version": "3.14.0", "name": "elastic-node"}}}}
{"metricset": {"samples":{"a":{"value":3.2}}, "timestamp": 1496170422281000}}`

func GeoIpLazyDownload() error {
	srv := apmservertest.NewUnstartedServer()
	if err := srv.Start(); err != nil {
		return err
	}

	serverURL := srv.URL + "/intake/v2/events"
	req, err := http.NewRequest("POST", serverURL, strings.NewReader(requestBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("X-Forwarded-For", "8.8.8.8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("query failed with status code: %d", resp.StatusCode)
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if err := waitGeoIPDownload(); err != nil {
		return err
	}

	// Clean all datastreams containing tags.
	cleanupElasticsearch()

	return nil
}

func waitGeoIPDownload() error {
	timer := time.NewTimer(2 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	defer timer.Stop()

	for {
		select {
		case <-ticker.C:
			resp, err := Elasticsearch.Ingest.GeoIPStats()
			if err != nil {
				return err
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
			var stats = struct {
				Stats struct {
					DatabasesCount int `json:"databases_count"`
				} `json:"stats"`
			}{}
			if err := json.Unmarshal(body, &stats); err != nil {
				return err
			}
			if stats.Stats.DatabasesCount == 3 {
				log.Println("GeoIP database downloaded")
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("download timeout exceeded")
		}
	}
}
