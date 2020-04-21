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

package model

import (
	"context"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

const (
	sourcemapProcessorName = "sourcemap"
	sourcemapDocType       = "sourcemap"
)

var (
	// TODO(axw) SourcemapMetrics should be unexported, but it's
	// being used for the decoder in processor/asset. We should
	// give that its own metrics registry, and unexport this one.

	SourcemapMetrics        = monitoring.Default.NewRegistry("apm-server.processor.sourcemap")
	sourcemapCounter        = monitoring.NewInt(SourcemapMetrics, "counter")
	sourcemapProcessorEntry = common.MapStr{"name": sourcemapProcessorName, "event": sourcemapDocType}
)

type Sourcemap struct {
	ServiceName    string
	ServiceVersion string
	Sourcemap      string
	BundleFilepath string
}

func (pa *Sourcemap) Transform(ctx context.Context, tctx *transform.Context) []beat.Event {
	sourcemapCounter.Inc()
	if pa == nil {
		return nil
	}

	if tctx.Config.SourcemapStore == nil {
		logp.NewLogger(logs.Sourcemap).Error("Sourcemap Accessor is nil, cache cannot be invalidated.")
	} else {
		tctx.Config.SourcemapStore.Added(ctx, pa.ServiceName, pa.ServiceVersion, pa.BundleFilepath)
	}

	ev := beat.Event{
		Fields: common.MapStr{
			"processor": sourcemapProcessorEntry,
			sourcemapDocType: common.MapStr{
				"bundle_filepath": utility.UrlPath(pa.BundleFilepath),
				"service":         common.MapStr{"name": pa.ServiceName, "version": pa.ServiceVersion},
				"sourcemap":       pa.Sourcemap,
			},
		},
		Timestamp: time.Now(),
	}
	return []beat.Event{ev}
}
