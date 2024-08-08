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
	"time"

	"go.elastic.co/fastjson"
)

type Metricset struct {
	Name     string            `json:"name,omitempty"`
	Interval string            `json:"interval,omitempty"`
	Samples  []MetricsetSample `json:"samples,omitempty"`
}

type MetricsetSample struct {
	Name      string
	Type      string
	Unit      string
	Histogram Histogram
	Summary   SummaryMetric
	Value     float64
}

type Histogram struct {
	Values []float64 `json:"values"`
	Counts []uint64  `json:"counts"`
}

func (h Histogram) isZero() bool {
	return len(h.Counts) == 0
}

type SummaryMetric struct {
	Count uint64  `json:"value_count"`
	Sum   float64 `json:"sum"`
}

func (s SummaryMetric) isZero() bool {
	return s.Count == 0
}

// TODO(axw) update ingest pipelines to map metric samples
// to the top level of the document.
func (ms *MetricsetSample) MarshalFastJSON(w *fastjson.Writer) error {
	w.RawString(`{"name":`)
	w.String(ms.Name)
	if ms.Type != "" {
		w.RawString(`,"type":`)
		w.String(ms.Type)
	}
	if ms.Unit != "" {
		w.RawString(`,"unit":`)
		w.String(ms.Unit)
	}
	switch ms.Type {
	case "histogram":
		w.RawString(`,"values":[`)
		for i, value := range ms.Histogram.Values {
			if i > 0 {
				w.RawByte(',')
			}
			w.Float64(value)
		}
		w.RawString(`],"counts":[`)
		for i, count := range ms.Histogram.Counts {
			if i > 0 {
				w.RawByte(',')
			}
			w.Int64(int64(count))
		}
		w.RawByte(']')
	case "summary":
		w.RawString(`,"value_count":`)
		w.Int64(int64(ms.Summary.Count))
		w.RawString(`,"sum":`)
		w.Float64(ms.Summary.Sum)
	default:
		w.RawString(`,"value":`)
		w.Float64(ms.Value)
	}
	w.RawByte('}')
	return nil
}

type AggregatedDuration struct {
	Count uint64
	Sum   time.Duration
}

func (d AggregatedDuration) isZero() bool {
	return d.Count == 0
}

func (d AggregatedDuration) MarshalFastJSON(w *fastjson.Writer) error {
	w.RawString(`{"count":`)
	w.Int64(int64(d.Count))
	w.RawString(`,"sum.us":`)
	w.Int64(d.Sum.Microseconds())
	w.RawByte('}')
	return nil
}
