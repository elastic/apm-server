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
	"io"
	"net/http"

	"github.com/elastic/beats/libbeat/logp"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/validation"

	"github.com/elastic/apm-server/decoder"
	er "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction"
)

var (
	errUnrecognizedObject    = errors.New("did not recognize object type")
	errInvalidMetadataFormat = errors.New("invalid metadata format")
)

const batchSize = 10

var models = []struct {
	key          string
	schema       *jsonschema.Schema
	modelDecoder func(interface{}, error) (transform.Transformable, error)
}{
	{
		"transaction",
		transaction.ModelSchema(),
		transaction.V2DecodeEvent,
	},
	{
		"span",
		span.ModelSchema(),
		span.V2DecodeEvent,
	},
	{
		"metric",
		metric.ModelSchema(),
		metric.DecodeMetric,
	},
	{
		"error",
		er.ModelSchema(),
		er.V2DecodeEvent,
	},
}

type v2Handler struct {
	requestDecoder decoder.ReqDecoder
	tconfig        transform.Config
}

// handleRawModel validates and decodes a single json object into its struct form
func (v *v2Handler) handleRawModel(rawModel map[string]interface{}) (transform.Transformable, error) {
	for _, model := range models {
		if entry, ok := rawModel[model.key]; ok {
			err := validation.Validate(entry, model.schema)
			if err != nil {
				return nil, err
			}

			tr, err := model.modelDecoder(entry, err)
			if err != nil {
				return nil, err
			}
			return tr, nil
		}
	}
	return nil, errUnrecognizedObject
}

// readBatch will read up to `batchSize` objects from the ndjson stream
// it returns a slice of eventables, a serverResponse and a bool that indicates if we're at EOF.
func (v *v2Handler) readBatch(batchSize int, reader *decoder.NDJSONStreamReader, response *streamResponse) ([]transform.Transformable, bool) {
	var err error
	var rawModel map[string]interface{}

	var eventables []transform.Transformable
	for i := 0; i < batchSize && err == nil; i++ {
		rawModel, err = reader.Read()
		if err != nil && err != io.EOF {
			switch e := err.(type) {
			case decoder.ReadError:
				response.addWithMessage(ServerError, 1, e.Error())
				// return early, we can't recover from a read error
				return eventables, true
			case decoder.JSONDecodeError:
				response.addWithOffendingDocument(InvalidJSONErr, e.Error(), reader.LastLine())
				response.Invalid++
			}
		}

		if rawModel != nil {
			tr, err := v.handleRawModel(rawModel)
			if err != nil {
				response.addWithOffendingDocument(SchemaValidationErr, err.Error(), reader.LastLine())
				response.Invalid++
				continue
			}
			eventables = append(eventables, tr)
		}
	}

	return eventables, reader.IsEOF()
}
func (v *v2Handler) readMetadata(r *http.Request, ndReader *decoder.NDJSONStreamReader) (*metadata.Metadata, error) {
	// first item is the metadata object
	rawModel, err := ndReader.Read()
	if err != nil {
		return nil, err
	}

	rawMetadata, ok := rawModel["metadata"].(map[string]interface{})
	if !ok {
		return nil, errUnrecognizedObject
	}
	// augment the metadata object with information from the request, like user-agent or remote address
	reqMeta, err := v.requestDecoder(r)
	if err != nil {
		return nil, err
	}

	for k, v := range reqMeta {
		utility.InsertInMap(rawMetadata, k, v.(map[string]interface{}))
	}

	// validate the metadata object against our jsonschema
	err = validation.Validate(rawMetadata, metadata.ModelSchema())
	if err != nil {
		return nil, err
	}

	// create a metadata struct
	metadata, err := metadata.DecodeMetadata(rawMetadata)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

func (v *v2Handler) handleRequestBody(r *http.Request, ndReader *decoder.NDJSONStreamReader, report reporter) *streamResponse {
	resp := &streamResponse{}

	metadata, err := v.readMetadata(r, ndReader)

	// no point in continuing if we couldn't read the metadata
	if err != nil {
		switch e := err.(type) {
		case decoder.ReadError:
			resp.addWithMessage(ServerError, 1, e.Error())
		case decoder.JSONDecodeError:
			resp.addWithOffendingDocument(InvalidJSONErr, err.Error(), ndReader.LastLine())
		default:
			resp.addWithOffendingDocument(SchemaValidationErr, err.Error(), ndReader.LastLine())
		}

		return resp
	}

	tctx := &transform.Context{
		RequestTime: requestTime(r),
		Config:      v.tconfig,
		Metadata:    *metadata,
	}

	for {
		transformables, done := v.readBatch(batchSize, ndReader, resp)
		if transformables != nil {
			err := report(r.Context(), pendingReq{
				transformables: transformables,
				tcontext:       tctx,
			})

			if err != nil {
				if err == errChannelClosed {
					resp.add(ShuttingDownErr, 1)
					return resp
				}

				if err == errFull {
					resp.add(QueueFullErr, len(transformables))
					resp.Dropped += len(transformables)
					continue
				}

				resp.addWithMessage(ServerError, len(transformables), err.Error())
			}
		}

		if done {
			break
		}
	}
	return resp
}

func (v *v2Handler) sendResponse(logger *logp.Logger, w http.ResponseWriter, sr *streamResponse) {
	statusCode := sr.statusCode()

	w.WriteHeader(statusCode)
	if statusCode != http.StatusAccepted {
		// this singals to the client that we're closing the connection
		// but also signals to http.Server that it should close it:
		// https://golang.org/src/net/http/server.go#L1254
		w.Header().Add("Connection", "Close")

		buf, err := sr.marshal()
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
	sr := streamResponse{
		Dropped:  -1,
		Accepted: -1,
		Invalid:  1,
	}
	sr.add(InvalidContentTypeErr, 1)

	v.sendResponse(requestLogger(r), w, &sr)
}

func (v *v2Handler) Handle(beaterConfig *Config, report reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := requestLogger(r)
		ndReader, err := decoder.NDJSONStreamDecodeCompressedWithLimit(r, beaterConfig.MaxUnzippedSize)
		if err != nil {
			// if we can't set up the ndjsonreader,
			// we won't be able to make sense of the body
			v.handleInvalidHeaders(w, r)
			return
		}

		streamResponse := v.handleRequestBody(r, ndReader, report)

		v.sendResponse(logger, w, streamResponse)
	})
}
