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
	"os"
	"testing"
	"time"
)

var stats struct {
	GeoIPStats `json:"stats"`
}

type GeoIPStats struct {
	DatabasesCount int `json:"databases_count"`
}

func TestMain(m *testing.M) {
	log.Println("INFO: starting stack containers...")
	initContainers()
	if err := StartStackContainers(); err != nil {
		log.Fatalf("failed to start stack containers: %v", err)
	}
	initElasticSearch()
	if err := waitGeoIPDownload(); err != nil {
		log.Fatalf("failed to download GeoIp database: %v", err)
	}
	initKibana()
	initSettings()
	initOTEL()
	log.Println("INFO: running system tests...")
	os.Exit(m.Run())
}

func waitGeoIPDownload() error {
	timer := time.NewTimer(30 * time.Second)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	first := true
	for {
		select {
		case <-ticker.C:
			resp, err := Elasticsearch.Ingest.GeoIPStats()
			if err != nil {
				return err
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			err = json.Unmarshal([]byte(body), &stats)
			if err != nil {
				return err
			}

			// [GeoLite2-ASN.mmdb, GeoLite2-City.mmdb, GeoLite2-Country.mmdb]
			if stats.DatabasesCount == 3 {
				log.Println("GeoIp database downloaded")
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("download timeout exceeded")
		}

		if first {
			log.Println("waiting for GeoIP database to download")
			first = false
		}
	}
}
