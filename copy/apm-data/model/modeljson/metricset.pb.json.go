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

package modeljson

import (
	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
)

var metricTypeText = map[modelpb.MetricType]string{
	modelpb.MetricType_METRIC_TYPE_GAUGE:     "gauge",
	modelpb.MetricType_METRIC_TYPE_COUNTER:   "counter",
	modelpb.MetricType_METRIC_TYPE_HISTOGRAM: "histogram",
	modelpb.MetricType_METRIC_TYPE_SUMMARY:   "summary",
}

func MetricsetModelJSON(me *modelpb.Metricset, out *modeljson.Metricset) {
	var samples []modeljson.MetricsetSample
	if n := len(me.Samples); n > 0 {
		samples = make([]modeljson.MetricsetSample, n)
		for i, sample := range me.Samples {
			if sample != nil {
				sampleJson := modeljson.MetricsetSample{
					Name:  sample.Name,
					Type:  metricTypeText[sample.Type],
					Unit:  sample.Unit,
					Value: sample.Value,
				}
				if sample.Histogram != nil {
					sampleJson.Histogram = modeljson.Histogram{
						Values: sample.Histogram.Values,
						Counts: sample.Histogram.Counts,
					}
				}
				if sample.Summary != nil {
					sampleJson.Summary = modeljson.SummaryMetric{
						Count: sample.Summary.Count,
						Sum:   sample.Summary.Sum,
					}
				}

				samples[i] = sampleJson
			}
		}
	}
	*out = modeljson.Metricset{
		Name:     me.Name,
		Interval: me.Interval,
		Samples:  samples,
	}
}
