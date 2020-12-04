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

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	"github.com/elastic/apm-server/model/modeldecoder/rumv3"
	v2 "github.com/elastic/apm-server/model/modeldecoder/v2"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
)

var (
	ErrUnrecognizedObject = errors.New("did not recognize object type")
)

const (
	batchSize = 10
)

type decodeMetadataFunc func(decoder.Decoder, *model.Metadata) error

type Processor struct {
	Mconfig          modeldecoder.Config
	MaxEventSize     int
	streamReaderPool sync.Pool
	decodeMetadata   decodeMetadataFunc
	isRUM            bool
}

func BackendProcessor(cfg *config.Config) *Processor {
	return &Processor{
		Mconfig:        modeldecoder.Config{Experimental: cfg.Mode == config.ModeExperimental},
		MaxEventSize:   cfg.MaxEventSize,
		decodeMetadata: v2.DecodeNestedMetadata,
		isRUM:          false,
	}
}

func RUMV2Processor(cfg *config.Config) *Processor {
	return &Processor{
		Mconfig:        modeldecoder.Config{Experimental: cfg.Mode == config.ModeExperimental},
		MaxEventSize:   cfg.MaxEventSize,
		decodeMetadata: v2.DecodeNestedMetadata,
		isRUM:          true,
	}
}

func RUMV3Processor(cfg *config.Config) *Processor {
	return &Processor{
		Mconfig:        modeldecoder.Config{Experimental: cfg.Mode == config.ModeExperimental},
		MaxEventSize:   cfg.MaxEventSize,
		decodeMetadata: rumv3.DecodeNestedMetadata,
		isRUM:          true,
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

// IdentifyEventType takes a reader and reads ahead the first key of the
// underlying json input. This method makes some assumptions met by the
// input format:
// - the input is in json format
// - every valid ndjson line only has one root key
func (p *Processor) IdentifyEventType(body []byte, result *Result) string {
	// find event type, trim spaces and account for single and double quotes
	body = bytes.TrimLeft(body, `{ "'`)
	end := bytes.Index(body, []byte(`"`))
	if end == -1 {
		end = bytes.Index(body, []byte(`'`))
	}
	if end == -1 {
		return ""
	}
	return string(body[0:end])
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
		body, err := reader.ReadAhead()
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
		if len(body) == 0 {
			// required for backwards compatibility - sending empty lines was permitted in previous versions
			continue
		}
		eventType := p.IdentifyEventType(body, response)
		input := modeldecoder.Input{
			RequestTime: requestTime,
			Metadata:    *streamMetadata,
			Config:      p.Mconfig,
		}
		switch eventType {
		case "error":
			var event model.Error
			err := v2.DecodeNestedError(reader, &input, &event)
			if handleDecodeErr(err, reader, response) {
				continue
			}
			event.RUM = p.isRUM
			batch.Errors = append(batch.Errors, &event)
		case "metricset":
			var event model.Metricset
			err := v2.DecodeNestedMetricset(reader, &input, &event)
			if handleDecodeErr(err, reader, response) {
				continue
			}
			batch.Metricsets = append(batch.Metricsets, &event)
		case "span":
			var event model.Span
			err := v2.DecodeNestedSpan(reader, &input, &event)
			if handleDecodeErr(err, reader, response) {
				continue
			}
			event.RUM = p.isRUM
			batch.Spans = append(batch.Spans, &event)
		case "transaction":
			var event model.Transaction
			err := v2.DecodeNestedTransaction(reader, &input, &event)
			if handleDecodeErr(err, reader, response) {
				continue
			}
			batch.Transactions = append(batch.Transactions, &event)
		case "e":
			var event model.Error
			err := rumv3.DecodeNestedError(reader, &input, &event)
			if handleDecodeErr(err, reader, response) {
				continue
			}
			event.RUM = p.isRUM
			batch.Errors = append(batch.Errors, &event)
		case "me":
			var event model.Metricset
			err := rumv3.DecodeNestedMetricset(reader, &input, &event)
			if handleDecodeErr(err, reader, response) {
				continue
			}
			batch.Metricsets = append(batch.Metricsets, &event)
		case "x":
			var event rumv3.Transaction
			err := rumv3.DecodeNestedTransaction(reader, &input, &event)
			if handleDecodeErr(err, reader, response) {
				continue
			}
			batch.Transactions = append(batch.Transactions, &event.Transaction)
			batch.Metricsets = append(batch.Metricsets, event.Metricsets...)
			for _, span := range event.Spans {
				span.RUM = true
				batch.Spans = append(batch.Spans, span)
			}
		default:
			response.LimitedAdd(&Error{
				Type:     InvalidInputErrType,
				Message:  errors.Wrap(ErrUnrecognizedObject, eventType).Error(),
				Document: string(reader.LatestLine()),
			})
			continue
		}
	}
	return reader.IsEOF()
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

	var e = err
	if err, ok := err.(modeldecoder.DecoderError); ok {
		e = err.Unwrap()
	}
	if errors.Is(e, decoder.ErrLineTooLong) {
		return &Error{
			Type:     InputTooLargeErrType,
			Message:  "event exceeded the permitted size.",
			Document: string(sr.LatestLine()),
		}
	}
	return err
}
