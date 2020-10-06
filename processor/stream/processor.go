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
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"go.elastic.co/apm"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/rumv3"
	v2 "github.com/elastic/apm-server/model/modeldecoder/v2"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
	"github.com/pkg/errors"
)

var (
	ErrUnrecognizedObject = errors.New("did not recognize object type")
)

const (
	batchSize = 10
)

type decodeMetadataFunc func(decoder.Decoder, *model.Metadata) error
type decodeTransactionFunc func(decoder.Decoder, *modeldecoder.Input, *model.Transaction) error
type decodeErrorFunc func(decoder.Decoder, *modeldecoder.Input, *model.Error) error
type decodeSpanFunc func(decoder.Decoder, *modeldecoder.Input, *model.Span) error

// functions with the decodeEventFunc signature decode their input argument into their batch argument (output)
type decodeEventFunc func(modeldecoder.Input, *model.Batch) error

type Processor struct {
	Mconfig           modeldecoder.Config
	MaxEventSize      int
	streamReaderPool  sync.Pool
	decodeMetadata    decodeMetadataFunc
	decodeError       decodeErrorFunc
	decodeSpan        decodeSpanFunc
	decodeTransaction decodeTransactionFunc
	models            map[string]decodeEventFunc
}

func BackendProcessor(cfg *config.Config) *Processor {
	return &Processor{
		Mconfig:           modeldecoder.Config{Experimental: cfg.Mode == config.ModeExperimental, RUM: false},
		MaxEventSize:      cfg.MaxEventSize,
		decodeMetadata:    v2.DecodeNestedMetadata,
		decodeError:       v2.DecodeNestedError,
		decodeSpan:        v2.DecodeNestedSpan,
		decodeTransaction: v2.DecodeNestedTransaction,
		models: map[string]decodeEventFunc{
			"metricset": modeldecoder.DecodeMetricset,
		},
	}
}

func RUMV2Processor(cfg *config.Config) *Processor {
	return &Processor{
		Mconfig:           modeldecoder.Config{Experimental: cfg.Mode == config.ModeExperimental, RUM: true},
		MaxEventSize:      cfg.MaxEventSize,
		decodeMetadata:    v2.DecodeNestedMetadata,
		decodeError:       v2.DecodeNestedError,
		decodeSpan:        v2.DecodeNestedSpan,
		decodeTransaction: v2.DecodeNestedTransaction,
		models: map[string]decodeEventFunc{
			"metricset": modeldecoder.DecodeRUMV2Metricset,
		},
	}
}

func RUMV3Processor(cfg *config.Config) *Processor {
	return &Processor{
		Mconfig:        modeldecoder.Config{Experimental: cfg.Mode == config.ModeExperimental, HasShortFieldNames: true, RUM: true},
		MaxEventSize:   cfg.MaxEventSize,
		decodeMetadata: rumv3.DecodeNestedMetadata,
		models: map[string]decodeEventFunc{
			"x":  modeldecoder.DecodeRUMV3Transaction,
			"e":  modeldecoder.DecodeRUMV3Error,
			"me": modeldecoder.DecodeRUMV3Metricset,
		},
	}
}

func (p *Processor) readMetadata(reader *streamReader, metadata *model.Metadata) error {
	if err := p.decodeMetadata(reader, metadata); err != nil {
		err = reader.wrapError(err)
		if err == io.EOF {
			return &Error{
				Type:     InvalidInputErrType,
				Message:  "EOF while reading metadata",
				Document: string(reader.LatestLine()),
			}
		}
		if _, ok := err.(*Error); ok {
			return err
		}
		return &Error{
			Type:     InvalidInputErrType,
			Message:  err.Error(),
			Document: string(reader.LatestLine()),
		}
	}
	return nil
}

func handleDecodeErr(err error, r *streamReader, result *Result) bool {
	if err == nil || err == io.EOF {
		return false
	}
	e, ok := err.(*Error)
	if !ok || (e.Type != InvalidInputErrType && e.Type != InputTooLargeErrType) {
		e = &Error{
			Type:     InvalidInputErrType,
			Message:  err.Error(),
			Document: string(r.LatestLine()),
		}
	}
	result.LimitedAdd(e)
	return true
}

// IdentifyEventType takes a reader and reads ahead the first key of the
// underlying json input. This method makes some assumptions met by the
// input format:
// - the input is in json format
// - every valid ndjson line only has one root key
func (p *Processor) IdentifyEventType(reader *decoder.NDJSONStreamDecoder, result *Result) (string, error) {
	body, err := reader.ReadAhead()
	if err != nil && err != io.EOF {
		return "", err
	}
	// find event type, trim spaces and account for single and double quotes
	body = bytes.TrimLeft(body, `{ "'`)
	end := bytes.Index(body, []byte(`"`))
	if end == -1 {
		end = bytes.Index(body, []byte(`'`))
	}
	if end == -1 {
		return "", err
	}
	return string(body[0:end]), err
}

// readBatch will read up to `batchSize` objects from the ndjson stream,
// returning a slice of Transformables and a boolean indicating that there
// might be more to read.
func (p *Processor) readBatch(
	ctx context.Context,
	ipRateLimiter *rate.Limiter,
	requestTime time.Time,
	streamMetadata *model.Metadata,
	batchSize int,
	batch *model.Batch,
	reader *streamReader,
	response *Result,
) bool {

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
			return true
		}
	}

	// input events are decoded and appended to the batch
	for i := 0; i < batchSize && !reader.IsEOF(); i++ {
		eventType, err := p.IdentifyEventType(reader.NDJSONStreamDecoder, response)
		if err != nil && err != io.EOF {
			err = reader.wrapError(err)
			if e, ok := err.(*Error); ok && (e.Type == InvalidInputErrType || e.Type == InputTooLargeErrType) {
				response.LimitedAdd(e)
				continue
			} else {
				// return early, we assume we can only recover from a input error types
				response.Add(err)
				return true
			}
		}
		if eventType == "" && err != nil && err == io.EOF {
			continue
		}
		input := modeldecoder.Input{
			RequestTime: requestTime,
			Metadata:    *streamMetadata,
			Config:      p.Mconfig,
		}
		if decodeFn, ok := p.models[eventType]; ok {
			var rawModel map[string]interface{}
			err = reader.Decode(&rawModel)
			err = reader.wrapError(err)
			if err != nil && err != io.EOF {
				if e, ok := err.(*Error); ok && (e.Type == InvalidInputErrType || e.Type == InputTooLargeErrType) {
					response.LimitedAdd(e)
					continue
				}
				// return early, we assume we can only recover from a input error types
				response.Add(err)
				return true
			}
			if len(rawModel) > 0 {
				input.Raw = rawModel[eventType]
				if err := decodeFn(input, batch); err != nil {
					response.LimitedAdd(&Error{
						Type:     InvalidInputErrType,
						Message:  err.Error(),
						Document: string(reader.LatestLine()),
					})
					continue
				}
			}
		} else {
			switch eventType {
			case "error":
				var event model.Error
				if handleDecodeErr(p.decodeError(reader, &input, &event), reader, response) {
					continue
				}
				batch.Errors = append(batch.Errors, &event)
			case "span":
				var event model.Span
				if handleDecodeErr(p.decodeSpan(reader, &input, &event), reader, response) {
					continue
				}
				batch.Spans = append(batch.Spans, &event)
			case "transaction":
				var event model.Transaction
				if handleDecodeErr(p.decodeTransaction(reader, &input, &event), reader, response) {
					continue
				}
				batch.Transactions = append(batch.Transactions, &event)
			default:
				response.LimitedAdd(&Error{
					Type:     InvalidInputErrType,
					Message:  errors.Wrap(ErrUnrecognizedObject, eventType).Error(),
					Document: string(reader.LatestLine()),
				})
				continue
			}
		}
	}
	return reader.IsEOF()
}

// HandleStream processes a stream of events
func (p *Processor) HandleStream(ctx context.Context, ipRateLimiter *rate.Limiter, meta *model.Metadata, reader io.Reader, report publish.Reporter) *Result {
	res := &Result{}

	sr := p.getStreamReader(reader)
	defer sr.release()

	// first item is the metadata object
	err := p.readMetadata(sr, meta)
	if err != nil {
		// no point in continuing if we couldn't read the metadata
		res.Add(err)
		return res
	}

	requestTime := utility.RequestTime(ctx)

	sp, ctx := apm.StartSpan(ctx, "Stream", "Reporter")
	defer sp.End()

	var batch model.Batch
	var done bool
	for !done {
		done = p.readBatch(ctx, ipRateLimiter, requestTime, meta, batchSize, &batch, sr, res)
		if batch.Len() == 0 {
			continue
		}
		// NOTE(axw) `report` takes ownership of transformables, which
		// means we cannot reuse the slice memory. We should investigate
		// alternative interfaces between the processor and publisher
		// which would enable better memory reuse.
		if err := report(ctx, publish.PendingReq{
			Transformables: batch.Transformables(),
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
		res.AddAccepted(batch.Len())
		batch.Reset()
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
		processor:           p,
		NDJSONStreamDecoder: decoder.NewNDJSONStreamDecoder(r, p.MaxEventSize),
	}
}

// streamReader wraps NDJSONStreamReader, converting errors to stream errors.
type streamReader struct {
	processor *Processor
	*decoder.NDJSONStreamDecoder
}

// release releases the streamReader, adding it to its Processor's sync.Pool.
// The streamReader must not be used after release returns.
func (sr *streamReader) release() {
	sr.Reset(nil)
	sr.processor.streamReaderPool.Put(sr)
}

func (sr *streamReader) wrapError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(decoder.JSONDecodeError); ok {
		return &Error{
			Type:     InvalidInputErrType,
			Message:  err.Error(),
			Document: string(sr.LatestLine()),
		}
	}
	if errors.Is(err, decoder.ErrLineTooLong) {
		return &Error{
			Type:     InputTooLargeErrType,
			Message:  "event exceeded the permitted size.",
			Document: string(sr.LatestLine()),
		}
	}
	return err
}
