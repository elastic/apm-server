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

package sourcemap

import (
	"strings"

	"go.elastic.co/apm"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	asset "github.com/elastic/apm-server/processor/asset/sourcemap"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/sourcemap"
)

var (
	// MonitoringMap holds a mapping for request.IDs to monitoring counters
	MonitoringMap = request.DefaultMonitoringMapForRegistry(registry)
	registry      = monitoring.Default.NewRegistry("apm-server.sourcemap", monitoring.PublishExpvar)
)

// Handler returns a request.Handler for managing asset requests.
func Handler(dec decoder.ReqDecoder, sourcemapStore *sourcemap.Store, report publish.Reporter) request.Handler {
	return func(c *request.Context) {
		if c.Request.Method != "POST" {
			c.Result.SetDefault(request.IDResponseErrorsMethodNotAllowed)
			c.Write()
			return
		}

		data, err := dec(c.Request)
		if err != nil {
			if strings.Contains(err.Error(), request.MapResultIDToStatus[request.IDResponseErrorsRequestTooLarge].Keyword) {
				c.Result.SetWithError(request.IDResponseErrorsRequestTooLarge, err)
			} else {
				c.Result.SetWithError(request.IDResponseErrorsDecode, err)
			}
			c.Write()
			return
		}

		if err = asset.Processor.Validate(data); err != nil {
			c.Result.SetWithError(request.IDResponseErrorsValidate, err)
			c.Write()
			return
		}

		transformables, err := asset.Processor.Decode(data, sourcemapStore)
		if err != nil {
			c.Result.SetWithError(request.IDResponseErrorsDecode, err)
			c.Write()
			return
		}

		req := publish.PendingReq{Transformables: transformables}
		span, ctx := apm.StartSpan(c.Request.Context(), "Send", "Reporter")
		defer span.End()
		req.Trace = !span.Dropped()

		if err = report(ctx, req); err != nil {
			if err == publish.ErrChannelClosed {
				c.Result.SetWithError(request.IDResponseErrorsShuttingDown, err)
			} else {
				c.Result.SetWithError(request.IDResponseErrorsFullQueue, err)
			}
			c.Write()
		}

		c.Result.SetDefault(request.IDResponseValidAccepted)
		c.Write()
	}
}
