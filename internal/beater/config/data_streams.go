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

package config

// DataStreamsConfig holds data streams configuration.
type DataStreamsConfig struct {
	Namespace string `config:"namespace"`

	// WaitForIntegration controls whether APM Server waits for the Fleet
	// integration package to be installed before indexing events.
	//
	// This config is ignored when running under Elastic Agent; it is intended
	// for running APM Server standalone, relying on Fleet to install the integration
	// for creating Elasticsearch index templates, ILM policies, and ingest pipelines.
	//
	// This configuration requires either a connection to Kibana or Elasticsearch.
	WaitForIntegration bool `config:"wait_for_integration"`
}

func defaultDataStreamsConfig() DataStreamsConfig {
	return DataStreamsConfig{
		Namespace:          "default",
		WaitForIntegration: true,
	}
}
