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

package modeljson

import (
	"testing"

	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestCloudToModelJSON(t *testing.T) {
	testCases := map[string]struct {
		proto    *modelpb.Cloud
		expected *modeljson.Cloud
	}{
		"empty": {
			proto:    &modelpb.Cloud{},
			expected: &modeljson.Cloud{},
		},
		"no pointers": {
			proto: &modelpb.Cloud{
				AccountId:        "accountid",
				AccountName:      "accountname",
				AvailabilityZone: "availabilityzone",
				InstanceId:       "instanceid",
				InstanceName:     "instancename",
				MachineType:      "machinetype",
				ProjectId:        "projectid",
				ProjectName:      "projectname",
				Provider:         "provider",
				Region:           "region",
				ServiceName:      "servicename",
			},
			expected: &modeljson.Cloud{
				AvailabilityZone: "availabilityzone",
				Provider:         "provider",
				Region:           "region",
				Account: modeljson.CloudAccount{
					ID:   "accountid",
					Name: "accountname",
				},
				Instance: modeljson.CloudInstance{
					ID:   "instanceid",
					Name: "instancename",
				},
				Machine: modeljson.CloudMachine{
					Type: "machinetype",
				},
				Project: modeljson.CloudProject{
					ID:   "projectid",
					Name: "projectname",
				},
				Service: modeljson.CloudService{
					Name: "servicename",
				},
			},
		},
		"full": {
			proto: &modelpb.Cloud{
				Origin: &modelpb.CloudOrigin{
					AccountId:   "origin_accountid",
					Provider:    "origin_provider",
					Region:      "origin_region",
					ServiceName: "origin_servicename",
				},
				AccountId:        "accountid",
				AccountName:      "accountname",
				AvailabilityZone: "availabilityzone",
				InstanceId:       "instanceid",
				InstanceName:     "instancename",
				MachineType:      "machinetype",
				ProjectId:        "projectid",
				ProjectName:      "projectname",
				Provider:         "provider",
				Region:           "region",
				ServiceName:      "servicename",
			},
			expected: &modeljson.Cloud{
				AvailabilityZone: "availabilityzone",
				Provider:         "provider",
				Region:           "region",
				Origin: modeljson.CloudOrigin{
					Account: modeljson.CloudAccount{
						ID: "origin_accountid",
						// TODO this will always be empty
						//Name: "origin_accountname",
					},
					Provider: "origin_provider",
					Region:   "origin_region",
					Service: modeljson.CloudService{
						Name: "origin_servicename",
					},
				},

				Account: modeljson.CloudAccount{
					ID:   "accountid",
					Name: "accountname",
				},
				Instance: modeljson.CloudInstance{
					ID:   "instanceid",
					Name: "instancename",
				},
				Machine: modeljson.CloudMachine{
					Type: "machinetype",
				},
				Project: modeljson.CloudProject{
					ID:   "projectid",
					Name: "projectname",
				},
				Service: modeljson.CloudService{
					Name: "servicename",
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var out modeljson.Cloud
			CloudModelJSON(tc.proto, &out)
			diff := cmp.Diff(*tc.expected, out)
			require.Empty(t, diff)
		})
	}
}
