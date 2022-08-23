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

package modelprocessor

import (
	"context"
	"fmt"

	"github.com/elastic/apm-server/internal/datastreams"
	"github.com/elastic/apm-server/internal/model"
)

// SetDataStream is a model.BatchProcessor that sets the data stream for events.
type SetDataStream struct {
	Namespace string
}

// ProcessBatch sets data stream fields for each event in b.
func (s *SetDataStream) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for i := range *b {
		(*b)[i].DataStream.Namespace = s.Namespace
		if (*b)[i].DataStream.Type == "" || (*b)[i].DataStream.Dataset == "" {
			s.setDataStream(&(*b)[i])
		}
	}
	return nil
}

func (s *SetDataStream) setDataStream(event *model.APMEvent) {
	switch event.Processor {
	case model.SpanProcessor, model.TransactionProcessor:
		event.DataStream.Type = datastreams.TracesType
		event.DataStream.Dataset = model.TracesDataset
		// In order to maintain different ILM policies, RUM traces are sent to
		// a different datastream.
		if isRUMAgentName(event.Agent.Name) {
			event.DataStream.Dataset = model.RUMTracesDataset
		}
	case model.ErrorProcessor:
		event.DataStream.Type = datastreams.LogsType
		event.DataStream.Dataset = model.ErrorsDataset
	case model.LogProcessor:
		event.DataStream.Type = datastreams.LogsType
		event.DataStream.Dataset = model.AppLogsDataset
	case model.MetricsetProcessor:
		event.DataStream.Type = datastreams.MetricsType
		event.DataStream.Dataset = metricsetDataset(event)
	}
}

func isRUMAgentName(agentName string) bool {
	switch agentName {
	// These are all the known agents that send "RUM" data to the APM Server.
	case "rum-js", "js-base", "iOS/swift":
		return true
	}
	return false
}

func metricsetDataset(event *model.APMEvent) string {
	if event.Transaction != nil || event.Span != nil || event.Service.Name == "" {
		// Metrics that include well-defined transaction/span fields
		// (i.e. breakdown metrics, transaction and span metrics) will
		// be stored separately from custom application metrics.
		return model.InternalMetricsDataset
	}

	if event.Metricset != nil {
		// Well-defined system/runtime metrics are stored in the shared
		// "internal" metrics data stream, as they are not application-specific.
		//
		// TODO(axw) agents should indicate that a metricset is internal.
		// If a metricset is identified as internal, then we'll ignore any
		// metrics that we don't already know about; otherwise they will end
		// up creating service-specific data streams.
		internal := true
		for name := range event.Metricset.Samples {
			if !isInternalMetricName(name) {
				internal = false
				break
			}
		}
		if internal {
			// The internal metrics data stream does not use dynamic
			// mapping, so we must drop type and unit if specified.
			for name, sample := range event.Metricset.Samples {
				if sample.Type == "" && sample.Unit == "" {
					continue
				}
				sample.Type = ""
				sample.Unit = ""
				event.Metricset.Samples[name] = sample
			}
			return model.InternalMetricsDataset
		}
	}

	// All other metrics are assumed to be application-specific metrics,
	// and so will be stored in an application-specific data stream.
	return fmt.Sprintf(
		"%s.%s", model.AppMetricsDataset,
		datastreams.NormalizeServiceName(event.Service.Name),
	)
}
