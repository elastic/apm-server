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
	"context"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

const (
	processorName  = "metric"
	docType        = "metric"
	transactionKey = "transaction"
	spanKey        = "span"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.metric")
	transformations = monitoring.NewInt(Metrics, "transformations")
	processorEntry  = common.MapStr{"name": processorName, "event": docType}
)

type Sample struct {
	Name  string
	Value float64
}

// Transaction provides enough information to connect a metricset to the related kind of transactions
type Transaction struct {
	Name *string
	Type *string
}

// Span provides enough information to connect a metricset to the related kind of spans
type Span struct {
	Type    *string
	Subtype *string
}

type Metricset struct {
	Metadata    metadata.Metadata
	Samples     []*Sample
	Labels      common.MapStr
	Transaction *Transaction
	Span        *Span
	Timestamp   time.Time
}

func (s *Span) fields() common.MapStr {
	if s == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Set(fields, "type", s.Type)
	utility.Set(fields, "subtype", s.Subtype)
	return fields
}

func (t *Transaction) fields() common.MapStr {
	if t == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Set(fields, "type", t.Type)
	utility.Set(fields, "name", t.Name)
	return fields
}

func (me *Metricset) Transform(ctx context.Context, tctx *transform.Context) []beat.Event {
	transformations.Inc()
	if me == nil {
		return nil
	}

	fields := common.MapStr{}
	for _, sample := range me.Samples {
		if _, err := fields.Put(sample.Name, sample.Value); err != nil {
			logp.NewLogger(logs.Transform).Warnf("failed to transform sample %#v", sample)
			continue
		}
	}

	fields["processor"] = processorEntry
	me.Metadata.Set(fields)

	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", me.Labels)
	utility.DeepUpdate(fields, transactionKey, me.Transaction.fields())
	utility.DeepUpdate(fields, spanKey, me.Span.fields())

	return []beat.Event{
		{
			Fields:    fields,
			Timestamp: me.Timestamp,
		},
	}
}
