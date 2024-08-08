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
	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
)

func CloudModelJSON(c *modelpb.Cloud, out *modeljson.Cloud) {
	*out = modeljson.Cloud{
		AvailabilityZone: c.AvailabilityZone,
		Provider:         c.Provider,
		Region:           c.Region,
		Account: modeljson.CloudAccount{
			ID:   c.AccountId,
			Name: c.AccountName,
		},
		Service: modeljson.CloudService{
			Name: c.ServiceName,
		},
		Project: modeljson.CloudProject{
			ID:   c.ProjectId,
			Name: c.ProjectName,
		},
		Instance: modeljson.CloudInstance{
			ID:   c.InstanceId,
			Name: c.InstanceName,
		},
		Machine: modeljson.CloudMachine{
			Type: c.MachineType,
		},
	}
	if c.Origin != nil {
		out.Origin = modeljson.CloudOrigin{
			Provider: c.Origin.Provider,
			Region:   c.Origin.Region,
			Account: modeljson.CloudAccount{
				ID: c.Origin.AccountId,
			},
			Service: modeljson.CloudService{
				Name: c.Origin.ServiceName,
			},
		}
	}

}
