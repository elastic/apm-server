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

import "github.com/elastic/apm-server/datastreams"

// DataStream identifies the data stream to which an event will be written.
type DataStream struct {
	// Type holds the data_stream.type identifier.
	Type string

	// Dataset holds the data_stream.dataset identifier.
	Dataset string

	// Namespace holds the data_stream.namespace identifier.
	Namespace string
}

func (d *DataStream) setFields(fields *mapStr) {
	fields.maybeSetString(datastreams.TypeField, d.Type)
	fields.maybeSetString(datastreams.DatasetField, d.Dataset)
	fields.maybeSetString(datastreams.NamespaceField, d.Namespace)
}
