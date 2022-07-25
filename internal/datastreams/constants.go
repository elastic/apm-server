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

package datastreams

// Constants for data stream types.
const (
	LogsType    = "logs"
	MetricsType = "metrics"
	TracesType  = "traces"
)

// Cosntants for data stream event metadata fields.
const (
	TypeField      = "data_stream.type"
	DatasetField   = "data_stream.dataset"
	NamespaceField = "data_stream.namespace"
)

// IndexFormat holds the variable "index" format to use for the libbeat Elasticsearch output.
// Each event the server publishes is expected to contain data_stream.* fields, which will be
// added to the documents as well as be used for routing documents to the correct data stream.
const IndexFormat = "%{[data_stream.type]}-%{[data_stream.dataset]}-%{[data_stream.namespace]}"
