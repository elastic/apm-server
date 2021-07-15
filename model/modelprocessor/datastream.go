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

	"github.com/elastic/apm-server/datastreams"
	"github.com/elastic/apm-server/model"
)

// SetDataStream is a model.BatchProcessor that sets the data stream for events.
type SetDataStream struct {
	Namespace string
}

// ProcessBatch sets data stream fields for each event in b.
func (s *SetDataStream) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for i := range *b {
		s.setDataStream(&(*b)[i])
	}
	return nil
}

func (s *SetDataStream) setDataStream(event *model.APMEvent) {
	switch {
	case event.Transaction != nil || event.Span != nil:
		event.DataStream.Type = datastreams.TracesType
		event.DataStream.Dataset = model.TracesDataset
	case event.Error != nil:
		event.DataStream.Type = datastreams.LogsType
		event.DataStream.Dataset = model.ErrorsDataset
	case event.Metricset != nil:
		event.DataStream.Type = datastreams.MetricsType
		// Metrics that include well-defined transaction/span fields
		// (i.e. breakdown metrics, transaction and span metrics) will
		// be stored separately from application and runtime metrics.
		event.DataStream.Dataset = model.InternalMetricsDataset
		if isApplicationMetricset(event.Metricset) {
			event.DataStream.Dataset = fmt.Sprintf(
				"%s.%s", model.AppMetricsDataset,
				datastreams.NormalizeServiceName(event.Metricset.Metadata.Service.Name),
			)
		}
	case event.ProfileSample != nil:
		event.DataStream.Type = datastreams.MetricsType
		event.DataStream.Dataset = model.ProfilesDataset
	}
	event.DataStream.Namespace = s.Namespace
}

func isApplicationMetricset(ms *model.Metricset) bool {
	return ms.Event == (model.MetricsetEventCategorization{}) &&
		ms.Transaction == (model.MetricsetTransaction{}) &&
		ms.Span == (model.MetricsetSpan{})
}
