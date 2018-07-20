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
	"time"

	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type sample struct {
	name  string
	value float64
}

type metric struct {
	samples   []*sample
	tags      common.MapStr
	timestamp time.Time
}

func (me *metric) Events(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	if me == nil {
		return nil
	}

	fields := common.MapStr{}
	for _, sample := range me.samples {
		if _, err := fields.Put(sample.name, sample.value); err != nil {
			logp.NewLogger("transform").Warnf("failed to transform sample %#v", sample)
			continue
		}
	}

	context := common.MapStr{}
	if me.tags != nil {
		context["tags"] = me.tags
	}

	fields["context"] = tctx.Metadata.Merge(context)
	fields["processor"] = processorEntry

	return []beat.Event{
		beat.Event{
			Fields:    fields,
			Timestamp: me.timestamp,
		},
	}
}

type metricDecoder struct {
	*utility.ManualDecoder
}
