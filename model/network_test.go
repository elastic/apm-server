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

func TestNetworkTransform(t *testing.T) {
	tests := []struct {
		Network Network
		Output  common.MapStr
	}{
		{
			Network: Network{},
			Output:  nil,
		},
		{
			Network: Network{
				Connection: NetworkConnection{
					Type:    "cell",
					Subtype: "LTE",
				},
				Carrier: NetworkCarrier{
					Name: "Vodafone",
					MCC:  "234",
					MNC:  "03",
					ICC:  "UK",
				},
			},
			Output: common.MapStr{
				"connection": common.MapStr{
					"type":    "cell",
					"subtype": "LTE",
				},
				"carrier": common.MapStr{
					"name": "Vodafone",
					"mcc":  "234",
					"mnc":  "03",
					"icc":  "UK",
				},
			},
		},
	}

	for _, test := range tests {
		output := test.Network.fields()
		assert.Equal(t, test.Output, output)
	}
}
