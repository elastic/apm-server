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
	"bufio"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/elastic/apm-server/beater/config"

	"github.com/elastic/apm-server/model"

	"github.com/santhosh-tekuri/jsonschema"
	"golang.org/x/time/rate"

	"go.elastic.co/apm"

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

type processorModel struct {
	schema       *jsonschema.Schema
	modelDecoder func(input interface{}, cfg model.Config, err error) (transform.Transformable, error)
}

type Processor struct {
	Tconfig      transform.Config
	Mconfig      model.Config
	MaxEventSize int
	bufferPool   sync.Pool
	models       map[string]processorModel
}

func BackendProcessor(cfg *config.Config) *Processor {
	return &Processor{
		Tconfig:      transform.Config{},
		Mconfig:      model.Config{Experimental: cfg.Mode == config.ModeExperimental},
		MaxEventSize: cfg.MaxEventSize,
		models: map[string]processorModel{
			"transaction": {
				schema:       transaction.ModelSchema(),
				modelDecoder: transaction.DecodeEvent,
			},
			"span": {
				schema:       span.ModelSchema(),
				modelDecoder: span.DecodeEvent,
			},
			"metricset": {
				schema:       metricset.ModelSchema(),
				modelDecoder: metricset.DecodeEvent,
			},
			"error": {
				er.ModelSchema(),
				er.DecodeEvent,
			},
		},
	}
}

func RUMProcessor(cfg *config.Config, tcfg *transform.Config) *Processor {
	return &Processor{
		Tconfig:      *tcfg,
		Mconfig:      model.Config{Experimental: cfg.Mode == config.ModeExperimental},
		MaxEventSize: cfg.MaxEventSize,
		models: map[string]processorModel{
			"transaction": {
				schema:       transaction.ModelSchema(),
				modelDecoder: transaction.DecodeEvent,
			},
			"span": {
				schema:       span.ModelSchema(),
				modelDecoder: span.DecodeEvent,
			},
			"metricset": {
				schema:       metricset.ModelSchema(),
				modelDecoder: metricset.DecodeEvent,
			},
			"error": {
				er.ModelSchema(),
				er.DecodeEvent,
			},
		},
	}
}

const batchSize = 10

func (p *Processor) readMetadata(reqMeta map[string]interface{}, reader StreamReader) (*metadata.Metadata, error) {
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
func (p *Processor) HandleRawModel(rawModel map[string]interface{}) (transform.Transformable, error) {
	for key, model := range p.models {
		if entry, ok := rawModel[key]; ok {
			err := validation.Validate(entry, model.schema)
			if err != nil {
				return nil, err
			}

			tr, err := model.modelDecoder(entry, p.Mconfig, err)
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
func (p *Processor) readBatch(ctx context.Context, ipRateLimiter *rate.Limiter, batchSize int, reader StreamReader, response *Result) ([]transform.Transformable, bool) {
	var (
		err        error
		rawModel   map[string]interface{}
		eventables []transform.Transformable
	)

	if ipRateLimiter != nil {
		// use provided rate limiter to throttle batch read
		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		err = ipRateLimiter.WaitN(ctxT, batchSize)
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
			tr, err := p.HandleRawModel(rawModel)
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

// HandleStream processes a stream of events
func (p *Processor) HandleStream(ctx context.Context, ipRateLimiter *rate.Limiter, meta map[string]interface{}, reader io.Reader, report publish.Reporter) *Result {
	res := &Result{}

	buf, ok := p.bufferPool.Get().(*bufio.Reader)
	if !ok {
		buf = bufio.NewReaderSize(reader, p.MaxEventSize)
	} else {
		buf.Reset(reader)
	}
	defer func() {
		buf.Reset(nil)
		p.bufferPool.Put(buf)
	}()

	lineReader := decoder.NewLineReader(buf, p.MaxEventSize)
	ndReader := decoder.NewNDJSONStreamReader(lineReader)

	// our own wrapper converts json reader errors to errors that are useful to us
	jsonReader := &srErrorWrapper{ndReader}

	metadata, err := p.readMetadata(meta, jsonReader)
	// no point in continuing if we couldn't read the metadata
	if err != nil {
		res.Add(err)
		return res
	}

	tctx := &transform.Context{
		RequestTime: utility.RequestTime(ctx),
		Config:      p.Tconfig,
		Metadata:    *metadata,
	}

	sp, ctx := apm.StartSpan(ctx, "Stream", "Reporter")
	defer sp.End()

	for {

		transformables, done := p.readBatch(ctx, ipRateLimiter, batchSize, jsonReader, res)
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
