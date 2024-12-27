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

package esclient

type Config struct {
	// ElasticsearchURL holds the Elasticsearch URL.
	ElasticsearchURL string

	// Username holds the Elasticsearch username for basic auth.
	Username string

	// Password holds the Elasticsearch password for basic auth.
	Password string

	// APIKey holds an Elasticsearch API Key.
	//
	// This will be set from $ELASTICSEARCH_API_KEY if specified.
	APIKey string

	// APMServerURL holds the APM Server URL.
	//
	// If this is unspecified, it will be derived from
	// ElasticsearchURL if that is an Elastic Cloud URL.
	APMServerURL string

	// KibanaURL holds the Kibana URL.
	//
	// If this is unspecified, it will be derived from
	// ElasticsearchURL if that is an Elastic Cloud URL.
	KibanaURL string

	// TLSSkipVerify determines if TLS certificate
	// verification is skipped or not. Default to false.
	//
	// If not specified the value will be take from
	// TLS_SKIP_VERIFY env var.
	// Any value different from "" is considered true.
	TLSSkipVerify bool
}

// NewConfig returns a Config intialised from environment variables.
// func NewConfig() (Config, error) {
// 	cfg := Config{}
// 	return cfg, err
// }
