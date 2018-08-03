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

package metric

import (
	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	Processor = &processor.EventsProcessor{
		PluralName:    "metrics",
		EventDecoder:  metric.DecodeMetric,
		PayloadSchema: metric.PayloadSchema(),
		DecodingCount: monitoring.NewInt(metric.Metrics, "decoding.count"),
		DecodingError: monitoring.NewInt(metric.Metrics, "decoding.errors"),
		ValidateCount: monitoring.NewInt(metric.Metrics, "validation.count"),
		ValidateError: monitoring.NewInt(metric.Metrics, "validation.errors"),
	}
)
