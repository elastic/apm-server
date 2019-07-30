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

package beater

import (
	"strings"

	"go.elastic.co/apm"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor/asset"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

func newAssetHandler(dec decoder.ReqDecoder, processor asset.Processor, cfg transform.Config, report publish.Reporter) request.Handler {
	return func(c *request.Context) {
		if c.Request.Method != "POST" {
			request.SendStatus(c, request.MethodNotAllowedResponse)
			return
		}

		data, err := dec(c.Request)
		if err != nil {
			if strings.Contains(err.Error(), "request body too large") {
				request.SendStatus(c, request.RequestTooLargeResponse)
				return
			}
			request.SendStatus(c, request.CannotDecodeResponse(err))
			return
		}

		if err = processor.Validate(data); err != nil {
			request.SendStatus(c, request.CannotValidateResponse(err))
			return
		}

		metadata, transformables, err := processor.Decode(data)
		if err != nil {
			request.SendStatus(c, request.CannotDecodeResponse(err))
			return
		}

		tctx := &transform.Context{
			RequestTime: utility.RequestTime(c.Request.Context()),
			Config:      cfg,
			Metadata:    *metadata,
		}

		req := publish.PendingReq{Transformables: transformables, Tcontext: tctx}
		ctx := c.Request.Context()
		span, ctx := apm.StartSpan(ctx, "Send", "Reporter")
		defer span.End()
		req.Trace = !span.Dropped()

		if err = report(ctx, req); err != nil {
			if err == publish.ErrChannelClosed {
				request.SendStatus(c, request.ServerShuttingDownResponse(err))
				return
			}
			request.SendStatus(c, request.FullQueueResponse(err))
			return
		}

		request.SendStatus(c, request.AcceptedResponse)
	}
}
