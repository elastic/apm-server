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

	"go.elastic.co/apm/v2"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/internal/decoder"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/internal/model"
	"github.com/elastic/apm-server/internal/model/modeldecoder"
	"github.com/elastic/apm-server/internal/model/modeldecoder/rumv3"
	v2 "github.com/elastic/apm-server/internal/model/modeldecoder/v2"
	"github.com/elastic/apm-server/internal/publish"
	"github.com/elastic/elastic-agent-libs/logp"
)

var (
	errUnrecognizedObject = errors.New("did not recognize object type")
)

const (
	errorEventType            = "error"
	metricsetEventType        = "metricset"
	spanEventType             = "span"
	transactionEventType      = "transaction"
	rumv3ErrorEventType       = "e"
	rumv3TransactionEventType = "x"
)

type decodeMetadataFunc func(decoder.Decoder, *model.APMEvent) error

// Processor decodes a streams and is safe for concurrent use. The processor
// accepts a channel that is used as a semaphore to control the maximum
// concurrent number of stream decode operations that can happen at any time.
// The buffered channel is meant to be shared between all the processors so
// the concurrency limit is shared between all the intake endpoints.
type Processor struct {
	streamReaderPool sync.Pool
	batchPool        sync.Pool
	decodeMetadata   decodeMetadataFunc
	sem              chan struct{}
	logger           *logp.Logger
	MaxEventSize     int
}

// Config holds configuration for Processor constructors.
type Config struct {
	// MaxEventSize holds the maximum event size, in bytes.
	MaxEventSize int

	// Semaphore holds a channel to which Processor.HandleStream
	// will send an item before proceeding, to limit concurrency.
	Semaphore chan struct{}
}

func BackendProcessor(cfg Config) *Processor {
	return &Processor{
		MaxEventSize:   cfg.MaxEventSize,
		decodeMetadata: v2.DecodeNestedMetadata,
		sem:            cfg.Semaphore,
		logger:         logp.NewLogger(logs.Processor),
	}
}

func RUMV2Processor(cfg Config) *Processor {
	return &Processor{
		MaxEventSize:   cfg.MaxEventSize,
		decodeMetadata: v2.DecodeNestedMetadata,
		sem:            cfg.Semaphore,
	}
}

func RUMV3Processor(cfg Config) *Processor {
	return &Processor{
		MaxEventSize:   cfg.MaxEventSize,
		decodeMetadata: rumv3.DecodeNestedMetadata,
		sem:            cfg.Semaphore,
	}
}

func (p *Processor) readMetadata(reader *streamReader, out *model.APMEvent) error {
	if err := p.decodeMetadata(reader, out); err != nil {
		err = reader.wrapError(err)
		if err == io.EOF {
			return &InvalidInputError{
				Message:  "EOF while reading metadata",
				Document: string(reader.LatestLine()),
			}
		}
		if _, ok := err.(*InvalidInputError); ok {
			return err
		}
		return &InvalidInputError{
			Message:  err.Error(),
			Document: string(reader.LatestLine()),
		}
	}
	return nil
}

// identifyEventType takes a reader and reads ahead the first key of the
// underlying json input. This method makes some assumptions met by the
// input format:
// - the input is in JSON format
// - every valid ndjson line only has one root key
// - the bytes that we must match on are ASCII
func (p *Processor) identifyEventType(body []byte) []byte {
	// find event type, trim spaces and account for single and double quotes
	var quote byte
	var key []byte
	for i, r := range body {
		if r == '"' || r == '\'' {
			quote = r
			key = body[i+1:]
			break
		}
	}
	end := bytes.IndexByte(key, quote)
	if end == -1 {
		return nil
	}
	return key[:end]
}

// readBatch reads up to `batchSize` events from the ndjson stream into
// batch, returning the number of events read and any error encountered.
// Callers should always process the n > 0 events returned before considering
// the error err.
func (p *Processor) readBatch(
	ctx context.Context,
	baseEvent model.APMEvent,
	batchSize int,
	batch *model.Batch,
	reader *streamReader,
	result *Result,
) (int, error) {

	// input events are decoded and appended to the batch
	origLen := len(*batch)
	for i := 0; i < batchSize && !reader.IsEOF(); i++ {
		body, err := reader.ReadAhead()
		if err != nil && err != io.EOF {
			err := reader.wrapError(err)
			var invalidInput *InvalidInputError
			if errors.As(err, &invalidInput) {
				result.LimitedAdd(err)
				continue
			}
			// return early, we assume we can only recover from a input error types
			return len(*batch) - origLen, err
		}
		if len(body) == 0 {
			// required for backwards compatibility - sending empty lines was permitted in previous versions
			continue
		}
		// We copy the event for each iteration of the batch, as to avoid
		// shallow copies of Labels and NumericLabels.
		input := modeldecoder.Input{Base: copyEvent(baseEvent)}
		switch eventType := p.identifyEventType(body); string(eventType) {
		case errorEventType:
			err = v2.DecodeNestedError(reader, &input, batch)
		case metricsetEventType:
			err = v2.DecodeNestedMetricset(reader, &input, batch)
		case spanEventType:
			err = v2.DecodeNestedSpan(reader, &input, batch)
		case transactionEventType:
			err = v2.DecodeNestedTransaction(reader, &input, batch)
		case rumv3ErrorEventType:
			err = rumv3.DecodeNestedError(reader, &input, batch)
		case rumv3TransactionEventType:
			err = rumv3.DecodeNestedTransaction(reader, &input, batch)
		default:
			err = errors.Wrap(errUnrecognizedObject, string(eventType))
		}
		if err != nil && err != io.EOF {
			result.LimitedAdd(&InvalidInputError{
				Message:  err.Error(),
				Document: string(reader.LatestLine()),
			})
		}
	}
	if reader.IsEOF() {
		return len(*batch) - origLen, io.EOF
	}
	return len(*batch) - origLen, nil
}

// HandleStream processes a stream of events in batches of batchSize at a time,
// updating result as events are accepted, or per-event errors occur.
//
// HandleStream will return an error when a terminal stream-level error occurs,
// such as the rate limit being exceeded, or due to authorization errors. In
// this case the result will only cover the subset of events accepted.
//
// Callers must not access result concurrently with HandleStream.
func (p *Processor) HandleStream(
	ctx context.Context,
	async bool,
	baseEvent model.APMEvent,
	reader io.Reader,
	batchSize int,
	processor model.BatchProcessor,
	result *Result,
) error {
	// Limits the number of concurrent batch decodes.
	// Defaults to 200 (N), only allowing N requests to read and cache Y events
	// (determined by batchSize) from the batch.
	// The ceiling may also reduce the contention on the modelindexer.activeMu.
	// Clients can set a async to true which makes the processor process the
	// events in the background. Returns with an error `publish.ErrFull` if the
	// semaphore is full. When asynchronous processing is requested, the batches
	// are decoded synchronously, but the batch is processed asynchronously.
	if err := p.acquireLock(ctx, async); err != nil {
		return err
	}
	sr := p.getStreamReader(reader)

	// first item is the metadata object
	if err := p.readMetadata(sr, &baseEvent); err != nil {
		// no point in continuing if we couldn't read the metadata
		sr.release()
		p.releaseLock()
		return err
	}

	defer func() {
		sr.release()
		if !async { // Release lock for sync processing.
			p.releaseLock()
		}
	}()

	sp, ctx := apm.StartSpan(ctx, "Stream", "Reporter")
	defer sp.End()

	var acquireLock bool
	for {
		// Async requests will re-acquire the lock after the first batch is
		// scheduled to be processed. If the semaphore is full, then we return
		// with `publish.ErrFull`.
		if async {
			if acquireLock {
				if err := p.acquireLock(ctx, async); err != nil {
					return err
				}
			} else {
				acquireLock = true
			}
		}
		var batch model.Batch
		if b, ok := p.batchPool.Get().(*model.Batch); ok {
			batch = (*b)[:0]
		}
		n, readErr := p.readBatch(ctx, baseEvent, batchSize, &batch, sr, result)
		if n > 0 {
			if async {
				go func() {
					// The semaphore is released once the batch is processed.
					defer p.releaseLock()
					err := processor.ProcessBatch(ctx, &batch)
					p.batchPool.Put(&batch)
					if err != nil && p.logger != nil {
						p.logger.Errorf("failed handling async request: %v", err)
					}
				}()
			} else {
				err := processor.ProcessBatch(ctx, &batch)
				p.batchPool.Put(&batch)
				if err != nil {
					return err
				}
				result.AddAccepted(n)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
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

func (p *Processor) acquireLock(ctx context.Context, async bool) error {
	select {
	case p.sem <- struct{}{}:
	default:
		if async {
			return publish.ErrFull
		}
		select {
		case p.sem <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (p *Processor) releaseLock() { <-p.sem }

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
		return &InvalidInputError{
			Message:  err.Error(),
			Document: string(sr.LatestLine()),
		}
	}

	var e = err
	if err, ok := err.(modeldecoder.DecoderError); ok {
		e = err.Unwrap()
	}
	if errors.Is(e, decoder.ErrLineTooLong) {
		return &InvalidInputError{
			TooLarge: true,
			Message:  "event exceeded the permitted size.",
			Document: string(sr.LatestLine()),
		}
	}
	return err
}

// copyEvent returns a shallow copy of the APMEvent with a deep copy of the
// labels and numeric labels.
func copyEvent(e model.APMEvent) model.APMEvent {
	var out = e
	if out.Labels != nil {
		out.Labels = out.Labels.Clone()
	}
	if out.NumericLabels != nil {
		out.NumericLabels = out.NumericLabels.Clone()
	}
	return out
}
