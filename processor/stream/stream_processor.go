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
	"time"

	"github.com/santhosh-tekuri/jsonschema"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-server/decoder"
	er "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/metricset"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

type rateLimiterKey struct{}

func ContextWithRateLimiter(ctx context.Context, limiter *rate.Limiter) context.Context {
	return context.WithValue(ctx, rateLimiterKey{}, limiter)
}

func rateLimiterFromContext(ctx context.Context) *rate.Limiter {
	if lim, ok := ctx.Value(rateLimiterKey{}).(*rate.Limiter); ok {
		return lim
	}
	return nil
}

var (
	ErrUnrecognizedObject = errors.New("did not recognize object type")
)

type StreamReader interface {
	Read() (map[string]interface{}, error)
	IsEOF() bool
	LatestLine() []byte
}

// srErrorWrapper wraps stream decoders and converts errors to
// something we know how to deal with
type srErrorWrapper struct {
	StreamReader
}

func (s *srErrorWrapper) Read() (map[string]interface{}, error) {
	v, err := s.StreamReader.Read()
	if err != nil {
		if _, ok := err.(decoder.JSONDecodeError); ok {
			return nil, &Error{
				Type:     InvalidInputErrType,
				Message:  err.Error(),
				Document: string(s.StreamReader.LatestLine()),
			}
		}

		if err == decoder.ErrLineTooLong {
			return nil, &Error{
				Type:     InputTooLargeErrType,
				Message:  "event exceeded the permitted size.",
				Document: string(s.StreamReader.LatestLine()),
			}
		}
	}
	return v, err
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
		"metricset",
		metricset.ModelSchema(),
		metricset.V2DecodeEvent,
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
		if err == io.EOF {
			return nil, &Error{
				Type:     InvalidInputErrType,
				Message:  "EOF while reading metadata",
				Document: string(reader.LatestLine()),
			}
		}
		return nil, err
	}

	rawMetadata, ok := rawModel["metadata"].(map[string]interface{})
	if !ok {
		return nil, &Error{
			Type:     InvalidInputErrType,
			Message:  ErrUnrecognizedObject.Error(),
			Document: string(reader.LatestLine()),
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
			Document: string(reader.LatestLine()),
		}
	}

	// create a metadata struct
	metadata, err := metadata.DecodeMetadata(rawMetadata)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

// HandleRawModel validates and decodes a single json object into its struct form
func (v *StreamProcessor) HandleRawModel(rawModel map[string]interface{}) (transform.Transformable, error) {
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
// it returns a slice of eventables and a bool that indicates if there might be more to read.
func (s *StreamProcessor) readBatch(ctx context.Context, rl *rate.Limiter, batchSize int, reader StreamReader, response *Result) ([]transform.Transformable, bool) {
	var (
		err        error
		rawModel   map[string]interface{}
		eventables []transform.Transformable
	)

	if rl != nil {
		// use provided rate limiter to throttle batch read
		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		err = rl.WaitN(ctxT, batchSize)
		cancel()
		if err != nil {
			response.Add(&Error{
				Type:    RateLimitErrType,
				Message: "rate limit exceeded",
			})
			return eventables, true
		}
	}

	for i := 0; i < batchSize && err == nil; i++ {

		rawModel, err = reader.Read()
		if err != nil && err != io.EOF {

			if e, ok := err.(*Error); ok && (e.Type == InvalidInputErrType || e.Type == InputTooLargeErrType) {
				response.LimitedAdd(e)
				continue
			}
			// return early, we assume we can only recover from a input error types
			response.Add(err)
			return eventables, true
		}

		if rawModel != nil {
			tr, err := s.HandleRawModel(rawModel)
			if err != nil {
				response.LimitedAdd(&Error{
					Type:     InvalidInputErrType,
					Message:  err.Error(),
					Document: string(reader.LatestLine()),
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

	// our own wrapper converts jsonreader errors to errors that are useful to us
	jsonReader = &srErrorWrapper{jsonReader}

	metadata, err := s.readMetadata(meta, jsonReader)
	// no point in continuing if we couldn't read the metadata
	if err != nil {
		res.Add(err)
		return res
	}

	tctx := &transform.Context{
		RequestTime: utility.RequestTime(ctx),
		Config:      s.Tconfig,
		Metadata:    *metadata,
	}
	rl := rateLimiterFromContext(ctx)

	sp, ctx := elasticapm.StartSpan(ctx, "Stream", "Reporter")
	defer sp.End()

	for {

		transformables, done := s.readBatch(ctx, rl, batchSize, jsonReader, res)
		if transformables != nil {
			err := report(ctx, publish.PendingReq{
				Transformables: transformables,
				Tcontext:       tctx,
				Trace:          !sp.Dropped(),
			})

			if err != nil {
				switch err {
				case publish.ErrChannelClosed:
					res.Add(&Error{
						Type:    ShuttingDownErrType,
						Message: "server is shutting down",
					})
				case publish.ErrFull:
					res.Add(&Error{
						Type:    QueueFullErrType,
						Message: err.Error(),
					})
				default:
					res.Add(err)
				}

				return res
			}

			res.AddAccepted(len(transformables))
		}

		if done {
			break
		}
	}
	return res
}
