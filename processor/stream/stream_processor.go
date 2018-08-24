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

func (v *StreamProcessor) readMetadata(reqMeta map[string]interface{}, ndReader StreamReader) (*metadata.Metadata, error) {
	// first item is the metadata object
	rawModel, err := ndReader.Read()
	if err != nil {
		return nil, err
	}

	rawMetadata, ok := rawModel["metadata"].(map[string]interface{})
	if !ok {
		return nil, ErrUnrecognizedObject
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
			switch e := err.(type) {
			case decoder.ReadError:
				response.AddWithMessage(ServerError, 1, e.Error())
				// return early, we can't recover from a read error
				return eventables, true
			case decoder.JSONDecodeError:
				response.AddWithOffendingDocument(InvalidJSONErr, e.Error(), reader.LastLine())
				response.Invalid++
			}
		}

		if rawModel != nil {
			tr, err := v.handleRawModel(rawModel)
			if err != nil {
				response.AddWithOffendingDocument(SchemaValidationErr, err.Error(), reader.LastLine())
				response.Invalid++
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
		switch e := err.(type) {
		case decoder.ReadError:
			res.AddWithMessage(ServerError, 1, e.Error())
		case decoder.JSONDecodeError:
			res.AddWithOffendingDocument(InvalidJSONErr, err.Error(), jsonReader.LastLine())
		default:
			res.AddWithOffendingDocument(SchemaValidationErr, err.Error(), jsonReader.LastLine())
		}

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
				if err == publish.ErrChannelClosed {
					res.Add(ShuttingDownErr, 1)
					return res
				}

				if err == publish.ErrFull {
					res.Add(QueueFullErr, len(transformables))
					res.Dropped += len(transformables)
					continue
				}

				res.AddWithMessage(ServerError, len(transformables), err.Error())
			}
		}

		if done {
			break
		}
	}
	return res
}
