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

package metricset

import (
	"github.com/elastic/apm-server/model/metricset"
	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	Processor = &processor.EventsProcessor{
		EventName:     "metricset",
		PayloadKey:    "metrics",
		EventDecoder:  metricset.V1DecodeEvent,
		PayloadSchema: metricset.PayloadSchema(),
		DecodingCount: monitoring.NewInt(metricset.Metrics, "decoding.count"),
		DecodingError: monitoring.NewInt(metricset.Metrics, "decoding.errors"),
		ValidateCount: monitoring.NewInt(metricset.Metrics, "validation.count"),
		ValidateError: monitoring.NewInt(metricset.Metrics, "validation.errors"),
	}
)
