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

	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
)

var compressionStrategyText = map[modelpb.CompressionStrategy]string{
	modelpb.CompressionStrategy_COMPRESSION_STRATEGY_EXACT_MATCH: "exact_match",
	modelpb.CompressionStrategy_COMPRESSION_STRATEGY_SAME_KIND:   "same_kind",
}

func DBModelJSON(db *modelpb.DB, out *modeljson.DB) {
	*out = modeljson.DB{
		Instance:     db.Instance,
		Statement:    db.Statement,
		Type:         db.Type,
		Link:         db.Link,
		RowsAffected: db.RowsAffected,
		User:         modeljson.DBUser{Name: db.UserName},
	}
}

func CompositeModelJSON(c *modelpb.Composite, out *modeljson.SpanComposite) {
	sumDuration := time.Duration(c.Sum * float64(time.Millisecond))
	*out = modeljson.SpanComposite{
		CompressionStrategy: compressionStrategyText[c.CompressionStrategy],
		Count:               int(c.Count),
		Sum:                 modeljson.SpanCompositeSum{US: sumDuration.Microseconds()},
	}
}

func SpanModelJSON(e *modelpb.Span, out *modeljson.Span) {
	*out = modeljson.Span{
		ID:                  e.Id,
		Name:                e.Name,
		Type:                e.Type,
		Subtype:             e.Subtype,
		Action:              e.Action,
		Kind:                e.Kind,
		Sync:                e.Sync,
		RepresentativeCount: e.RepresentativeCount,
	}
	if e.SelfTime != nil {
		out.SelfTime = modeljson.AggregatedDuration{
			Count: e.SelfTime.Count,
			Sum:   time.Duration(e.SelfTime.Sum),
		}
	}
	if e.Db != nil {
		out.DB = &modeljson.DB{}
		DBModelJSON(e.Db, out.DB)
	}
	if e.Message != nil {
		out.Message = &modeljson.Message{}
		MessageModelJSON(e.Message, out.Message)
	}
	if e.Composite != nil {
		out.Composite = &modeljson.SpanComposite{}
		CompositeModelJSON(e.Composite, out.Composite)
	}
	if e.DestinationService != nil {
		out.Destination = &modeljson.SpanDestination{
			Service: modeljson.SpanDestinationService{
				Type:     e.DestinationService.Type,
				Name:     e.DestinationService.Name,
				Resource: e.DestinationService.Resource,
			},
		}
		if e.DestinationService.ResponseTime != nil {
			out.Destination.Service.ResponseTime = modeljson.AggregatedDuration{
				Count: e.DestinationService.ResponseTime.Count,
				Sum:   time.Duration(e.DestinationService.ResponseTime.Sum),
			}
		}
	}

	if n := len(e.Links); n > 0 {
		out.Links = make([]modeljson.SpanLink, n)
		for i, link := range e.Links {
			out.Links[i] = modeljson.SpanLink{
				Trace: modeljson.SpanLinkTrace{ID: link.TraceId},
				Span:  modeljson.SpanLinkSpan{ID: link.SpanId},
			}
		}
	}
	if n := len(e.Stacktrace); n > 0 {
		out.Stacktrace = make([]modeljson.StacktraceFrame, n)
		for i, frame := range e.Stacktrace {
			StacktraceFrameModelJSON(frame, &out.Stacktrace[i])
		}
	}
}
