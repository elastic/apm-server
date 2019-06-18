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
	"net/http"
	"strings"

	"go.elastic.co/apm"

	"github.com/elastic/apm-server/beater/internal"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor/asset"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/server"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

type assetHandler struct {
	requestDecoder decoder.ReqDecoder
	processor      asset.Processor
	tconfig        transform.Config
}

func (h *assetHandler) Handle(beaterConfig *Config, report publish.Reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := h.processRequest(r, report)
		internal.SendCnt(w, r, res)
	})
}

func (h *assetHandler) processRequest(r *http.Request, report publish.Reporter) server.Response {
	if r.Method != "POST" {
		return server.MethodNotAllowed()
	}

	data, err := h.requestDecoder(r)
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return server.Error{err, http.StatusRequestEntityTooLarge}
		}
		return server.Error{err, http.StatusBadRequest}
	}

	if err = h.processor.Validate(data); err != nil {
		return server.Error{err, http.StatusBadRequest}
	}

	metadata, transformables, err := h.processor.Decode(data)
	if err != nil {
		return server.Error{err, http.StatusBadRequest}
	}

	tctx := &transform.Context{
		RequestTime: utility.RequestTime(r.Context()),
		Config:      h.tconfig,
		Metadata:    *metadata,
	}

	req := publish.PendingReq{Transformables: transformables, Tcontext: tctx}
	ctx := r.Context()
	span, ctx := apm.StartSpan(ctx, "send", "Reporter")
	defer span.End()
	req.Trace = !span.Dropped()

	if err = report(ctx, req); err != nil {
		if err == publish.ErrChannelClosed {
			return server.ShuttingDown()
		}
		return server.Error{err, http.StatusServiceUnavailable}
	}

	return server.Result{StatusCode: http.StatusAccepted}
}
