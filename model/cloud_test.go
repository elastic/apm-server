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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestCloudFields(t *testing.T) {
	tests := []struct {
		Cloud  Cloud
		Output common.MapStr
	}{
		{
			Cloud:  Cloud{},
			Output: nil,
		},
		{
			Cloud: Cloud{
				AvailabilityZone: "australia-southeast1-a",
				AccountID:        "acct123",
				AccountName:      "my-dev-account",
				InstanceID:       "inst-foo123xyz",
				InstanceName:     "my-instance",
				MachineType:      "n1-highcpu-96",
				ProjectID:        "snazzy-bobsled-123",
				ProjectName:      "Development",
				Provider:         "gcp",
				Region:           "australia-southeast1",
			},
			Output: common.MapStr{
				"availability_zone": "australia-southeast1-a",
				"account": common.MapStr{
					"id":   "acct123",
					"name": "my-dev-account",
				},
				"instance": common.MapStr{
					"id":   "inst-foo123xyz",
					"name": "my-instance",
				},
				"machine": common.MapStr{
					"type": "n1-highcpu-96",
				},
				"project": common.MapStr{
					"id":   "snazzy-bobsled-123",
					"name": "Development",
				},
				"provider": "gcp",
				"region":   "australia-southeast1",
			},
		},
	}

	for _, test := range tests {
		output := test.Cloud.fields()
		assert.Equal(t, test.Output, output)
	}
}
