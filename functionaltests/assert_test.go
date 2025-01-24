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

package functionaltests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
)

const (
	// elasticAgentOnCloudPolicyID is the ID of the default Elastic Agent policy
	// in Elastic Cloud that has APM package policy.
	elasticAgentOnCloudPolicyID = "policy-elastic-agent-on-cloud"
	// elasticAgentPackagePolicyAPM is the ID of the package policy within the Elastic Agent
	// on Cloud policy that enables APM by default.
	elasticAgentPackagePolicyAPM = "elastic-cloud-apm"
)

// assertElasticAPMEnabled verifies if APM is still enabled in the Elastic Cloud policy.
func assertElasticAPMEnabled(t *testing.T, policies kbclient.AgentPoliciesResponse) {
	t.Helper()

	policyFound := false
	packagePolicyFound := false
	for _, i := range policies.Items {
		if i.ID == elasticAgentOnCloudPolicyID {
			policyFound = true

			for _, pp := range i.PackagePolicies {
				if pp.ID == elasticAgentPackagePolicyAPM {
					packagePolicyFound = true
				}
			}

			break
		}
	}
	require.True(t, policyFound)
	require.True(t, packagePolicyFound)
}
