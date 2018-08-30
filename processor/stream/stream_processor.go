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

package stream

import (
	"context"
	"errors"
	"io"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/decoder"
	er "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	ErrUnrecognizedObject = errors.New("did not recognize object type")
)

type StreamReader interface {
	Read() (map[string]interface{}, error)
	IsEOF() bool
	LastLine() []byte
}

type StreamProcessor struct {
	Tconfig transform.Config
}

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

func (v *StreamProcessor) readMetadata(reqMeta map[string]interface{}, reader StreamReader) (*metadata.Metadata, error) {
	// first item is the metadata object
	rawModel, err := reader.Read()
	if err != nil {
		if e, ok := err.(decoder.JSONDecodeError); ok || err == io.EOF {
			return nil, &Error{
				Type:     InvalidInputErrType,
				Message:  e.Error(),
				Document: string(reader.LastLine()),
			}
		}
		return nil, err
	}

	rawMetadata, ok := rawModel["metadata"].(map[string]interface{})
	if !ok {
		return nil, &Error{
			Type:     InvalidInputErrType,
			Message:  ErrUnrecognizedObject.Error(),
			Document: string(reader.LastLine()),
		}
	}

	for k, v := range reqMeta {
		utility.InsertInMap(rawMetadata, k, v.(map[string]interface{}))
	}

	// validate the metadata object against our jsonschema
	err = validation.Validate(rawMetadata, metadata.ModelSchema())
	if err != nil {
		return nil, &Error{
			Type:     InvalidInputErrType,
			Message:  err.Error(),
			Document: string(reader.LastLine()),
		}
	}

	// create a metadata struct
	metadata, err := metadata.DecodeMetadata(rawMetadata)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

// handleRawModel validates and decodes a single json object into its struct form
func (v *StreamProcessor) handleRawModel(rawModel map[string]interface{}) (transform.Transformable, error) {
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
	return nil, ErrUnrecognizedObject
}

// readBatch will read up to `batchSize` objects from the ndjson stream
// it returns a slice of eventables, a serverResponse and a bool that indicates if we're at EOF.
func (v *StreamProcessor) readBatch(batchSize int, reader StreamReader, response *Result) ([]transform.Transformable, bool) {
	var err error
	var rawModel map[string]interface{}

	var eventables []transform.Transformable
	for i := 0; i < batchSize && err == nil; i++ {
		rawModel, err = reader.Read()
		if err != nil && err != io.EOF {

			if e, ok := err.(decoder.JSONDecodeError); ok {
				response.LimitedAdd(&Error{
					Type:     InvalidInputErrType,
					Message:  e.Error(),
					Document: string(reader.LastLine()),
				})
				continue
			}

			// return early, we assume we can only recover from a JSON decode error
			response.LimitedAdd(err)
			return eventables, true
		}

		if rawModel != nil {
			tr, err := v.handleRawModel(rawModel)
			if err != nil {
				response.LimitedAdd(&Error{
					Type:     InvalidInputErrType,
					Message:  err.Error(),
					Document: string(reader.LastLine()),
				})
				continue
			}
			eventables = append(eventables, tr)
		}
	}

	return eventables, reader.IsEOF()
}

func (s *StreamProcessor) HandleStream(ctx context.Context, meta map[string]interface{}, jsonReader StreamReader, report publish.Reporter) *Result {
	res := &Result{}
	metadata, err := s.readMetadata(meta, jsonReader)
	// no point in continuing if we couldn't read the metadata
	if err != nil {
		res.LimitedAdd(err)
		return res
	}

	tctx := &transform.Context{
		RequestTime: utility.RequestTime(ctx),
		Config:      s.Tconfig,
		Metadata:    *metadata,
	}

	for {
		transformables, done := s.readBatch(batchSize, jsonReader, res)
		if transformables != nil {
			err := report(ctx, publish.PendingReq{
				Transformables: transformables,
				Tcontext:       tctx,
			})

			if err != nil {
				switch err {
				case publish.ErrChannelClosed:
					res.LimitedAdd(&Error{
						Type:    ShuttingDownErrType,
						Message: "server is shutting down",
					})
				case publish.ErrFull:
					res.LimitedAdd(&Error{
						Type:    QueueFullErrType,
						Message: err.Error(),
					})
				default:
					res.LimitedAdd(err)
				}

				return res
			}

			res.Accepted += len(transformables)
		}

		if done {
			break
		}
	}
	return res
}
