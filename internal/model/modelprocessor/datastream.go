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
	"strings"

	"github.com/elastic/apm-data/model"
)

const (
	logsType       = "logs"
	appLogsDataset = "apm.app"
	errorsDataset  = "apm.error"

	metricsType            = "metrics"
	appMetricsDataset      = "apm.app"
	internalMetricsDataset = "apm.internal"

	tracesType       = "traces"
	tracesDataset    = "apm"
	rumTracesDataset = "apm.rum"
)

// SetDataStream is a model.BatchProcessor that routes events to the appropriate
// data streams.
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
		event.DataStream.Type = tracesType
		event.DataStream.Dataset = tracesDataset
		// In order to maintain different ILM policies, RUM traces are sent to
		// a different datastream.
		if isRUMAgentName(event.Agent.Name) {
			event.DataStream.Dataset = rumTracesDataset
		}
	case model.ErrorProcessor:
		event.DataStream.Type = logsType
		event.DataStream.Dataset = errorsDataset
	case model.LogProcessor:
		event.DataStream.Type = logsType
		event.DataStream.Dataset = appLogsDataset
	case model.MetricsetProcessor:
		event.DataStream.Type = metricsType
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
		if event.Event.Duration <= 0 {
			return internalMetricsDataset
		}
		var suffix string
		if duration := event.Event.Duration.Minutes(); duration >= 1 {
			suffix = fmt.Sprintf("%s.%.0fm", event.Metricset.Name, duration)
		} else {
			suffix = fmt.Sprintf("%s.%.0fs", event.Metricset.Name, event.Event.Duration.Seconds())
		}
		var dataset strings.Builder
		dataset.Grow(len(internalMetricsDataset) + 1 + len(suffix) + len(event.Metricset.Name))
		dataset.WriteString(internalMetricsDataset)
		dataset.WriteByte('.')
		dataset.WriteString(suffix)
		return dataset.String()
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
		for _, s := range event.Metricset.Samples {
			if !isInternalMetricName(s.Name) {
				internal = false
				break
			}
		}
		if internal {
			// The internal metrics data stream does not use dynamic
			// mapping, so we must drop type and unit if specified.
			for i, sample := range event.Metricset.Samples {
				if sample.Type == "" && sample.Unit == "" {
					continue
				}
				sample.Type = ""
				sample.Unit = ""
				event.Metricset.Samples[i] = sample
			}
			return internalMetricsDataset
		}
	}

	// All other metrics are assumed to be application-specific metrics,
	// and so will be stored in an application-specific data stream.
	suffix := normalizeServiceName(event.Service.Name)
	var dataset strings.Builder
	dataset.Grow(len(appMetricsDataset) + 1 + len(suffix))
	dataset.WriteString(appMetricsDataset)
	dataset.WriteByte('.')
	dataset.WriteString(suffix)
	return dataset.String()
}

// normalizeServiceName translates serviceName into a string suitable
// for inclusion in a data stream name.
//
// Concretely, this function will lowercase the string and replace any
// reserved characters with "_".
func normalizeServiceName(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(replaceReservedRune, s)
	return s
}

func replaceReservedRune(r rune) rune {
	switch r {
	case '\\', '/', '*', '?', '"', '<', '>', '|', ' ', ',', '#', ':':
		// These characters are not permitted in data stream names
		// by Elasticsearch.
		return '_'
	case '-':
		// Hyphens are used to separate the data stream type, dataset,
		// and namespace.
		return '_'
	}
	return r
}
