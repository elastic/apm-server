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

// AssetHandler returns a request.Handler for managing asset requests.
func AssetHandler(dec decoder.ReqDecoder, processor asset.Processor, cfg transform.Config, report publish.Reporter) request.Handler {
	return func(c *request.Context) {
		if c.Request.Method != "POST" {
			c.Result.SetDefault(request.IDResponseErrorsMethodNotAllowed)
			c.Write()
			return
		}

		data, err := dec(c.Request)
		if err != nil {
			if strings.Contains(err.Error(), request.KeywordResponseErrorsRequestTooLarge) {
				c.Result.SetDefault(request.IDResponseErrorsRequestTooLarge)
			} else {
				c.Result.SetDefault(request.IDResponseErrorsDecode)
			}
			c.Result.Err = err
			c.Write()
			return
		}

		if err = processor.Validate(data); err != nil {
			c.Result.SetWithError(request.IDResponseErrorsValidate, err)
			c.Write()
			return
		}

		metadata, transformables, err := processor.Decode(data)
		if err != nil {
			c.Result.SetWithError(request.IDResponseErrorsDecode, err)
			c.Write()
			return
		}

		tctx := &transform.Context{
			RequestTime: utility.RequestTime(c.Request.Context()),
			Config:      cfg,
			Metadata:    *metadata,
		}
		req := publish.PendingReq{Transformables: transformables, Tcontext: tctx}
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
