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

package elasticsearch

import (
	"context"

	"github.com/elastic/beats/v7/libbeat/licenser"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

// GetLicense gets the Elasticsearch licensing information.
func GetLicense(ctx context.Context, client Client) (licenser.License, error) {
	var result struct {
		License licenser.License `json:"license"`
	}
	req := esapi.LicenseGetRequest{}
	if err := doRequest(ctx, client, req, &result); err != nil {
		return licenser.License{}, err
	}
	return result.License, nil
}
