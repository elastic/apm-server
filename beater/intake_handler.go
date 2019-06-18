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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/apm-server/beater/internal"

	"github.com/elastic/apm-server/server"

	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
)

type intakeHandler struct {
	requestDecoder  decoder.ReqDecoder
	streamProcessor *stream.Processor
	rlc             *rlCache
}

func validateRequest(r *http.Request) *server.Error {
	if r.Method != "POST" {
		return server.MethodNotAllowed()
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "application/x-ndjson") {
		return &server.Error{errors.New(
			fmt.Sprintf("invalid content type: '%s'", r.Header.Get("Content-Type"))),
			http.StatusUnsupportedMediaType,
		}
	}
	return nil
}

func (v *intakeHandler) rateLimit(r *http.Request) (*rate.Limiter, *server.Error) {
	if rl, ok := v.rlc.getRateLimiter(utility.RemoteAddr(r)); ok {
		if !rl.Allow() {
			return nil, server.RateLimited()
		}
		return rl, nil
	}
	return nil, nil
}

func bodyReader(r *http.Request) (io.ReadCloser, *server.Error) {
	reader, err := decoder.CompressedRequestReader(r)
	if err != nil {
		return nil, &server.Error{err, http.StatusBadRequest}
	}
	return reader, nil
}

func (v *intakeHandler) Handle(beaterConfig *Config, report publish.Reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		serr := validateRequest(r)
		if serr != nil {
			internal.SendCnt(w, r, serr)
			return
		}

		rl, serr := v.rateLimit(r)
		if serr != nil {
			internal.SendCnt(w, r, serr)
			return
		}

		reader, serr := bodyReader(r)
		if serr != nil {
			internal.SendCnt(w, r, serr)
			return
		}

		// extract metadata information from the request, like user-agent or remote address
		reqMeta, err := v.requestDecoder(r)
		if err != nil {
			internal.SendCnt(w, r, server.Error{err, http.StatusBadRequest})
			return
		}

		sr := v.streamProcessor.HandleStream(r.Context(), rl, reqMeta, reader, report)
		if _, verbose := r.URL.Query()["verbose"]; verbose || sr.IsError() {
			internal.SendCnt(w, r, sr)
		} else {
			internal.SendCnt(w, r, server.Result{StatusCode: sr.Code()})
		}

	})
}
