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
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/elastic/apm-server/utility"

	"github.com/elastic/apm-server/transform"
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/validation"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/decoder"
	er "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction"
)

const batchSize = 10

func (v v2Route) Handler(beaterConfig *Config, report reporter) http.Handler {
	reqDecoder := v.configurableDecoder(
		beaterConfig,
		func(*http.Request) (map[string]interface{}, error) { return map[string]interface{}{}, nil },
	)

	v2Handler := v2Handler{
		requestDecoder: reqDecoder,
		tconfig:        v.transformConfig(beaterConfig),
	}

	return v.wrappingHandler(beaterConfig, v2Handler.Handle(beaterConfig, report))
}

var models = []struct {
	key          string
	schema       *jsonschema.Schema
	modelDecoder func(interface{}, error) (transform.Transformable, error)
}{
	{
		"transaction",
		transaction.ModelSchema(),
		transaction.DecodeEvent,
	},
	{
		"span",
		span.ModelSchema(),
		span.DecodeSpan,
	},
	{
		"metric",
		metric.ModelSchema(),
		metric.DecodeMetric,
	},
	{
		"error",
		er.ModelSchema(),
		er.DecodeEvent,
	},
}

type v2Handler struct {
	requestDecoder decoder.ReqDecoder
	tconfig        transform.Config
}

// handleRawModel validates and decodes a single json object into its struct form
func (v *v2Handler) handleRawModel(rawModel map[string]interface{}) (transform.Transformable, serverResponse) {
	for _, model := range models {
		if entry, ok := rawModel[model.key]; ok {
			err := validation.Validate(entry, model.schema)
			if err != nil {
				return nil, cannotValidateResponse(err)
			}

			tr, err := model.modelDecoder(entry, err)
			if err != nil {
				return tr, cannotDecodeResponse(err)
			}
			return tr, serverResponse{}
		}
	}
	return nil, cannotValidateResponse(errors.New("did not recognize object type"))
}

// readBatch will read up to `batchSize` objects from the ndjson stream
// it returns a slice of eventables, a serverResponse and a bool that indicates if we're at EOF.
func (v *v2Handler) readBatch(batchSize int, reader *decoder.NDJSONStreamReader, response *streamResponse) ([]transform.Transformable, bool) {
	var err error
	var rawModel map[string]interface{}

	eventables := []transform.Transformable{}
	for i := 0; i < batchSize && err == nil; i++ {
		rawModel, err = reader.Read()
		if err != nil && err != io.EOF {
			response.addError(cannotDecodeResponse(err))
			response.Invalid++
		}

		if rawModel != nil {
			tr, resp := v.handleRawModel(rawModel)
			if resp.IsError() {
				response.addError(resp)
				response.Invalid++
			}
			eventables = append(eventables, tr)
		}
	}

	return eventables, reader.IsEOF()
}
func (v *v2Handler) readMetadata(r *http.Request, ndjsonReader *decoder.NDJSONStreamReader) (*metadata.Metadata, serverResponse) {
	// first item is the metadata object
	rawData, err := ndjsonReader.Read()
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}

	rawMetadata, ok := rawData["metadata"].(map[string]interface{})
	if !ok {
		return nil, cannotValidateResponse(errors.New("invalid metadata format"))
	}

	// augment the metadata object with information from the request, like user-agent or remote address
	reqMeta, err := v.requestDecoder(r)
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}

	for k, v := range reqMeta {
		utility.MergeAdd(rawMetadata, k, v.(map[string]interface{}))
	}

	// validate the metadata object against our jsonschema
	err = validation.Validate(rawMetadata, metadata.ModelSchema())
	if err != nil {
		return nil, cannotValidateResponse(err)
	}

	// create a metadata struct
	metadata, err := metadata.DecodeMetadata(rawMetadata)
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}
	return metadata, serverResponse{}
}

func (v *v2Handler) handleRequest(r *http.Request, ndjsonReader *decoder.NDJSONStreamReader, report reporter) *streamResponse {
	resp := &streamResponse{}

	metadata, serverResponse := v.readMetadata(r, ndjsonReader)

	// no point in continueing if we couldn't read the metadata
	if serverResponse.IsError() {
		sr := streamResponse{}
		sr.addError(serverResponse)
		return &sr
	}

	tctx := &transform.Context{
		Config:   v.tconfig,
		Metadata: *metadata,
	}

	for {
		transformables, eof := v.readBatch(batchSize, ndjsonReader, resp)
		if transformables != nil {
			err := report(r.Context(), pendingReq{
				transformables: transformables,
				tcontext:       tctx,
			})
			if err != nil {
				if strings.Contains(err.Error(), "publisher is being stopped") {
					sr := streamResponse{}
					sr.addError(serverShuttingDownResponse(err))
					return &sr
				}

				resp.addErrorCount(fullQueueResponse(err), len(transformables))
				resp.Dropped += len(transformables)
			}
		}

		if eof {
			break
		}
	}
	return resp
}

func (v *v2Handler) sendResponse(w http.ResponseWriter, streamResponse *streamResponse) error {
	statusCode := http.StatusAccepted
	for k := range streamResponse.Errors {
		if k > statusCode {
			statusCode = k
		}
	}
	log.Println(statusCode, streamResponse)
	w.WriteHeader(statusCode)
	if statusCode != http.StatusAccepted {
		return json.NewEncoder(w).Encode(streamResponse)
	}
	return nil
}

func (v *v2Handler) Handle(beaterConfig *Config, report reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ndjsonReader, err := decoder.StreamDecodeLimitJSONData(r, beaterConfig.MaxUnzippedSize)
		if err != nil {
			sr := streamResponse{
				// we wont look at the body if the content type is wrong
				Dropped:  -1,
				Accepted: -1,
				Invalid:  -1,
			}
			sr.addError(cannotDecodeResponse(err))

			discardBuf := make([]byte, 2048)
			var err error
			for err != nil {
				_, err = r.Body.Read(discardBuf)
			}

			if err := v.sendResponse(w, &sr); err != nil {
				// trouble
			}
			return
		}

		streamResponse := v.handleRequest(r, ndjsonReader, report)

		// did we return early?
		if !ndjsonReader.IsEOF() {
			dropped, err := ndjsonReader.SkipToEnd()
			if err != io.EOF {
				// trouble
			}
			streamResponse.Dropped += dropped
		}

		if err := v.sendResponse(w, streamResponse); err != nil {
			// trouble
		}
	})
}
