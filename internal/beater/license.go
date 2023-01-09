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
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/elastic/beats/v7/libbeat/licenser"

	"github.com/elastic/go-elasticsearch/v8"
)

// getElasticsearchLicense gets the Elasticsearch licensing information.
func getElasticsearchLicense(ctx context.Context, client *elasticsearch.Client) (licenser.License, error) {
	resp, err := client.License.Get(client.License.Get.WithContext(ctx))
	if err != nil {
		return licenser.License{}, err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		body, _ := io.ReadAll(resp.Body)
		return licenser.License{}, fmt.Errorf("failed to get license (%s): %s", resp.Status(), body)
	}

	var result struct {
		License licenser.License `json:"license"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return licenser.License{}, err
	}
	return result.License, nil
}
