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

	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"

	"github.com/elastic/beats/libbeat/logp"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/decoder"
)

var (
	errInvalidMetadataFormat = errors.New("invalid metadata format")
)

type v2Handler struct {
	requestDecoder  decoder.ReqDecoder
	streamProcessor *stream.StreamProcessor
}

func (v *v2Handler) sendResponse(logger *logp.Logger, w http.ResponseWriter, sr *stream.Result) {
	statusCode := sr.StatusCode()

	w.WriteHeader(statusCode)
	if statusCode != http.StatusAccepted {
		// this singals to the client that we're closing the connection
		// but also signals to http.Server that it should close it:
		// https://golang.org/src/net/http/server.go#L1254
		w.Header().Add("Connection", "Close")

		buf, err := sr.Marshal()
		if err != nil {
			logger.Errorw("error sending response", "error", err)
		}
		_, err = w.Write(buf)
		if err != nil {
			logger.Errorw("error sending response", "error", err)
		}
		logger.Infow("error handling request", "error", sr.String())
	}
}

// handleInvalidHeaders reads out the rest of the body and discards it
// then returns an error response
func (v *v2Handler) handleInvalidHeaders(w http.ResponseWriter, r *http.Request) {
	sr := stream.Result{
		Dropped:  -1,
		Accepted: -1,
		Invalid:  1,
	}
	sr.Add(stream.InvalidContentTypeErr, 1)

	v.sendResponse(requestLogger(r), w, &sr)
}

func (v *v2Handler) Handle(beaterConfig *Config, report publish.Reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := requestLogger(r)
		ndReader, err := decoder.NDJSONStreamDecodeCompressedWithLimit(r, beaterConfig.MaxUnzippedSize)
		if err != nil {
			// if we can't set up the ndjsonreader,
			// we won't be able to make sense of the body
			v.handleInvalidHeaders(w, r)
			return
		}

		// extract metadata information from the request, like user-agent or remote address
		reqMeta, err := v.requestDecoder(r)
		if err != nil {
			sr := stream.Result{}
			sr.AddWithMessage(stream.ServerError, 1, err.Error())
			v.sendResponse(logger, w, &sr)
		}

		res := v.streamProcessor.HandleStream(r.Context(), reqMeta, ndReader, report)

		v.sendResponse(logger, w, res)
	})
}
