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

package beater

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/elastic/beats/libbeat/logp"
)

func insecureClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: tr}
}

func isServerUp(secure bool, host string, numRetries int, retryInterval time.Duration) bool {
	client := insecureClient()
	var check = func() bool {
		url := url.URL{Scheme: "http", Host: host, Path: "/"}
		if secure {
			url.Scheme = "https"
		}
		res, err := client.Get(url.String())
		return err == nil && res.StatusCode == 200
	}

	for i := 0; i <= numRetries; i++ {
		if check() {
			logp.NewLogger("http_client").Info("HTTP Server ready")
			return true
		}
		time.Sleep(retryInterval)
	}
	return false
}
