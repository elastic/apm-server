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

package telemetry

import (
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"io"
	"net/http"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.telemetry")
)

type HandlerConfig struct {
	// TelemetryUrl contains the URL to send telemetry to
	TelemetryUrl string

	// ClusterId contains the elastic cloud cluster ID
	ClusterId string

	// Version contains the APM Server version
	Version string
}

func Handler(cfg HandlerConfig) request.Handler {

	return func(c *request.Context) {
		err := sendTelemetry(cfg.TelemetryUrl, c.Request.Header.Get("content-type"), c.Request.Body, cfg.ClusterId, cfg.Version)
		if err != nil {
			return
		}
		c.Result.SetDefault(request.IDResponseValidOK)
		c.WriteResult()
	}

}

func sendTelemetry(url string, contentType string, body io.Reader, clusterId string, version string) error {

	client := &http.Client{}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}

	req.Header.Add("content-type", contentType)
	req.Header.Add("X-Elastic-Cluster-ID", clusterId)
	req.Header.Add("X-Elastic-Stack-Version", version)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	io.ReadAll(resp.Body)
	defer resp.Body.Close()
	return nil
}
