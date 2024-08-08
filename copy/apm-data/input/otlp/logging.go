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

package otlp

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var (
	jsonTracesMarshaler  = &ptrace.JSONMarshaler{}
	jsonMetricsMarshaler = &pmetric.JSONMarshaler{}
	jsonLogsMarshaler    = &plog.JSONMarshaler{}
)

type (
	tracesStringer  ptrace.Traces
	metricsStringer pmetric.Metrics
	logsStringer    plog.Logs
)

func (traces tracesStringer) String() string {
	data, err := jsonTracesMarshaler.MarshalTraces(ptrace.Traces(traces))
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (metrics metricsStringer) String() string {
	data, err := jsonMetricsMarshaler.MarshalMetrics(pmetric.Metrics(metrics))
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (logs logsStringer) String() string {
	data, err := jsonLogsMarshaler.MarshalLogs(plog.Logs(logs))
	if err != nil {
		return err.Error()
	}
	return string(data)
}
