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

const (
	TargetQA   = "qa"
	TargetProd = "pro"
)

// RegionFrom returns the appropriate region to run test
// against based on specified target.
// https://www.elastic.co/guide/en/cloud/current/ec-regions-templates-instances.html
func RegionFrom(target string) string {
	switch target {
	case TargetQA:
		return "aws-eu-west-1"
	case TargetProd:
		return "gcp-us-west2"
	default:
		panic("target value is not accepted")
	}
}

// EndpointFrom returns the appropriate endpoint for Elastic Cloud
// based on specified target.
func EndpointFrom(target string) string {
	switch target {
	case TargetQA:
		return "https://public-api.qa.cld.elstc.co"
	case TargetProd:
		return "https://api.elastic-cloud.com"
	default:
		panic("target value is not accepted")
	}
}

// DeploymentTemplateFrom returns the appropriate deployment template
// based on specified region.
func DeploymentTemplateFrom(region string) string {
	switch region {
	case "aws-eu-west-1":
		return "aws-storage-optimized"
	case "gcp-us-west2":
		return "gcp-storage-optimized"
	default:
		panic("region value is not accepted")
	}
}

const (
	// ManagedByDSL is the constant string used by Elasticsearch to specify that an Index is managed by Data Stream Lifecycle management.
	ManagedByDSL = "Data stream lifecycle"
	// ManagedByILM is the constant string used by Elasticsearch to specify that an Index is managed by Index Lifecycle Management.
	ManagedByILM = "Index Lifecycle Management"
)
