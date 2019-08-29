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

package api

import (
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/common"
)

type enrollResponse struct {
	BaseResponse
	AccessToken string `json:"item"`
}

func (e *enrollResponse) Validate() error {
	if !e.Success || len(e.AccessToken) == 0 {
		return errors.New("empty access_token")
	}
	return nil
}

// Enroll a beat in central management, this call returns a valid access token to retrieve
// configurations
func (c *Client) Enroll(
	beatType, beatName, beatVersion, hostname string,
	beatUUID uuid.UUID,
	enrollmentToken string,
) (string, error) {
	params := common.MapStr{
		"type":      beatType,
		"name":      beatName,
		"version":   beatVersion,
		"host_name": hostname,
	}

	resp := enrollResponse{}

	headers := http.Header{}
	headers.Set("kbn-beats-enrollment-token", enrollmentToken)

	_, err := c.request("POST", "/api/beats/agent/"+beatUUID.String(), params, headers, &resp)
	if err != nil {
		return "", err
	}

	if err := resp.Validate(); err != nil {
		return "", err
	}

	return resp.AccessToken, err
}
