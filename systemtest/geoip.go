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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

var (
	waitGeoIpInterval = 500 * time.Millisecond
	waitGeoIpTimeout  = 30 * time.Second
)

var stats struct {
	GeoIPStats `json:"stats"`
}

type GeoIPStats struct {
	DatabasesCount int `json:"databases_count"`
}

func DownloadGeoIPDatabase(t testing.TB) {
	b, err := json.Marshal(map[string]any{
		"persistent": map[string]any{
			"ingest.geoip.downloader.enabled":        true,
			"ingest.geoip.downloader.eager.download": true,
		},
	})
	require.NoError(t, err)

	r := esapi.ClusterPutSettingsRequest{
		Body: bytes.NewReader(b),
	}
	res, err := r.Do(context.Background(), Elasticsearch)
	require.NoError(t, err)
	defer res.Body.Close()
	require.False(t, res.IsError())

	err = waitGeoIPDownload()
	require.NoError(t, err)
}

func DeleteGeoIpDatabase(t testing.TB) {
	b, err := json.Marshal(map[string]any{
		"persistent": map[string]any{
			"ingest.geoip.downloader.enabled": false,
		},
	})
	require.NoError(t, err)

	r := esapi.ClusterPutSettingsRequest{
		Body: bytes.NewReader(b),
	}
	res, err := r.Do(context.Background(), Elasticsearch)
	require.NoError(t, err)
	defer res.Body.Close()
	require.False(t, res.IsError())

	err = waitGeoIPDeleted()
	require.NoError(t, err)
}

func waitGeoIPDownload() error {
	timer := time.NewTimer(waitGeoIpTimeout)

	ticker := time.NewTicker(waitGeoIpInterval)
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

func waitGeoIPDeleted() error {
	timer := time.NewTimer(waitGeoIpTimeout)

	ticker := time.NewTicker(waitGeoIpInterval)
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

			if stats.DatabasesCount == 0 {
				log.Println("GeoIp database deleted")
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("delete timeout exceeded")
		}

		if first {
			log.Println("waiting for GeoIP database to deleted")
			first = false
		}
	}
}
