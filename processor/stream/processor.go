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
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema"
	"golang.org/x/time/rate"

	"go.elastic.co/apm"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	er "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/field"
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

const (
	batchSize = 10
)

type processorModel struct {
	schema       *jsonschema.Schema
	modelDecoder func(model.Input) (transform.Transformable, error)
}

type Processor struct {
	Tconfig          transform.Config
	Mconfig          model.Config
	MaxEventSize     int
	streamReaderPool sync.Pool
	models           map[string]processorModel
	metadataSchema   *jsonschema.Schema
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
				schema:       er.ModelSchema(),
				modelDecoder: er.DecodeEvent,
			},
		},
		metadataSchema: metadata.ModelSchema(),
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
				schema:       er.ModelSchema(),
				modelDecoder: er.DecodeEvent,
			},
		},
		metadataSchema: metadata.ModelSchema(),
	}
}

func RUMV3Processor(cfg *config.Config, tcfg *transform.Config) *Processor {
	return &Processor{
		Tconfig:      *tcfg,
		Mconfig:      model.Config{Experimental: cfg.Mode == config.ModeExperimental, HasShortFieldNames: true},
		MaxEventSize: cfg.MaxEventSize,
		models: map[string]processorModel{
			"x": {
				schema:       transaction.RUMV3Schema,
				modelDecoder: transaction.DecodeRUMV3Event,
			},
			"y": {
				schema:       span.RUMV3Schema,
				modelDecoder: span.DecodeRUMV3Event,
			},
			"e": {
				schema:       er.RUMV3Schema,
				modelDecoder: er.DecodeRUMV3Event,
			},
		},
		metadataSchema: metadata.RUMV3ModelSchema(),
	}
}

func (p *Processor) readMetadata(reqMeta map[string]interface{}, reader *streamReader) (*metadata.Metadata, error) {
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

	fieldName := field.Mapper(p.Mconfig.HasShortFieldNames)
	rawMetadata, ok := rawModel[fieldName("metadata")].(map[string]interface{})
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

	if err := validation.Validate(rawMetadata, p.metadataSchema); err != nil {
		return nil, &Error{
			Type:     InvalidInputErrType,
			Message:  err.Error(),
			Document: string(reader.LatestLine()),
		}
	}
	return metadata.DecodeMetadata(rawMetadata, p.Mconfig.HasShortFieldNames)
}

// HandleRawModel validates and decodes a single json object into its struct form
func (p *Processor) HandleRawModel(rawModel map[string]interface{}, requestTime time.Time, streamMetadata metadata.Metadata) (transform.Transformable, error) {
	for key, m := range p.models {
		if entry, ok := rawModel[key]; ok {
			err := validation.Validate(entry, m.schema)
			if err != nil {
				return nil, err
			}

			tr, err := m.modelDecoder(model.Input{
				Raw:         entry,
				RequestTime: requestTime,
				Metadata:    streamMetadata,
				Config:      p.Mconfig,
			})
			if err != nil {
				return nil, err
			}
			return tr, nil
		}
	}
	return nil, ErrUnrecognizedObject
}

// readBatch will read up to `batchSize` objects from the ndjson stream,
// returning a slice of Transformables and a boolean indicating that there
// might be more to read.
func (p *Processor) readBatch(
	ctx context.Context,
	ipRateLimiter *rate.Limiter,
	requestTime time.Time,
	streamMetadata *metadata.Metadata,
	batchSize int,
	reader *streamReader,
	response *Result,
) ([]transform.Transformable, bool) {

	if ipRateLimiter != nil {
		// use provided rate limiter to throttle batch read
		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		err := ipRateLimiter.WaitN(ctxT, batchSize)
		cancel()
		if err != nil {
			response.Add(&Error{
				Type:    RateLimitErrType,
				Message: "rate limit exceeded",
			})
			return nil, true
		}
	}

	var out []transform.Transformable
	for i := 0; i < batchSize && !reader.IsEOF(); i++ {
		rawModel, err := reader.Read()
		if err != nil && err != io.EOF {
			if e, ok := err.(*Error); ok && (e.Type == InvalidInputErrType || e.Type == InputTooLargeErrType) {
				response.LimitedAdd(e)
				continue
			}
			// return early, we assume we can only recover from a input error types
			response.Add(err)
			return out, true
		}
		if len(rawModel) > 0 {
			tr, err := p.HandleRawModel(rawModel, requestTime, *streamMetadata)
			if err != nil {
				response.LimitedAdd(&Error{
					Type:     InvalidInputErrType,
					Message:  err.Error(),
					Document: string(reader.LatestLine()),
				})
				continue
			}
			out = append(out, tr)
		}
	}
	return out, reader.IsEOF()
}

// HandleStream processes a stream of events
func (p *Processor) HandleStream(ctx context.Context, ipRateLimiter *rate.Limiter, meta map[string]interface{}, reader io.Reader, report publish.Reporter) *Result {
	res := &Result{}

	sr := p.getStreamReader(reader)
	defer sr.release()

	// first item is the metadata object
	metadata, err := p.readMetadata(meta, sr)
	if err != nil {
		// no point in continuing if we couldn't read the metadata
		res.Add(err)
		return res
	}

	requestTime := utility.RequestTime(ctx)
	tctx := &transform.Context{Config: p.Tconfig}

	sp, ctx := apm.StartSpan(ctx, "Stream", "Reporter")
	defer sp.End()

	var transformables []transform.Transformable
	var done bool
	for !done {
		transformables, done = p.readBatch(ctx, ipRateLimiter, requestTime, metadata, batchSize, sr, res)
		if len(transformables) == 0 {
			continue
		}
		// NOTE(axw) `report` takes ownership of transformables, which
		// means we cannot reuse the slice memory. We should investigate
		// alternative interfaces between the processor and publisher
		// which would enable better memory reuse.
		if err := report(ctx, publish.PendingReq{
			Transformables: transformables,
			Tcontext:       tctx,
			Trace:          !sp.Dropped(),
		}); err != nil {
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
	return res
}

// getStreamReader returns a streamReader that reads ND-JSON lines from r.
func (p *Processor) getStreamReader(r io.Reader) *streamReader {
	if sr, ok := p.streamReaderPool.Get().(*streamReader); ok {
		sr.Reset(r)
		return sr
	}
	return &streamReader{
		processor:          p,
		NDJSONStreamReader: decoder.NewNDJSONStreamReader(r, p.MaxEventSize),
	}
}

// streamReader wraps NDJSONStreamReader, converting errors to stream errors.
type streamReader struct {
	processor *Processor
	*decoder.NDJSONStreamReader
}

// release releases the streamReader, adding it to its Processor's sync.Pool.
// The streamReader must not be used after release returns.
func (sr *streamReader) release() {
	sr.Reset(nil)
	sr.processor.streamReaderPool.Put(sr)
}

func (sr *streamReader) Read() (map[string]interface{}, error) {
	// TODO(axw) decode into a reused map, clearing out the
	// map between reads. We would require that decoders copy
	// any contents of rawModel that they wish to retain after
	// the call, in order to safely reuse the map.
	v, err := sr.NDJSONStreamReader.Read()
	if err != nil {
		if _, ok := err.(decoder.JSONDecodeError); ok {
			return nil, &Error{
				Type:     InvalidInputErrType,
				Message:  err.Error(),
				Document: string(sr.LatestLine()),
			}
		}
		if err == decoder.ErrLineTooLong {
			return nil, &Error{
				Type:     InputTooLargeErrType,
				Message:  "event exceeded the permitted size.",
				Document: string(sr.LatestLine()),
			}
		}
	}
	return v, err
}
