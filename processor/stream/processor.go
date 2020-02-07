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
	"regexp"
	"sync"
	"time"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model/transformer"

	"golang.org/x/time/rate"

	"go.elastic.co/apm"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	ErrUnrecognizedObject = errors.New("did not recognize object type")
)

func RUMProcessor(experimental bool, maxEventSize int, cfg *config.RumConfig) (*Processor, error) {
	sourcemapStore, err := cfg.MemoizedSourcemapStore()
	if err != nil {
		return nil, err
	}
	transformer := &transformer.Transformer{
		Experimental:        experimental,
		SourcemapStore:      sourcemapStore,
		LibraryPattern:      regexp.MustCompile(cfg.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(cfg.ExcludeFromGrouping),
	}
	return &Processor{
		Decoders: map[string]EventDecoder{
			"transaction": DecoderFunc(transformer.DecodeTransaction),
			"span":        DecoderFunc(transformer.DecodeSpan),
			"error":       DecoderFunc(transformer.DecodeError),
			"metricset":   DecoderFunc(transformer.DecodeMetricset),
		},
		MaxEventSize: maxEventSize,
	}, nil
}

func BackendProcessor(experimental bool, maxEventSize int) *Processor {
	transformer := transformer.Transformer{
		Experimental: experimental,
	}
	return &Processor{
		Decoders: map[string]EventDecoder{
			"transaction": DecoderFunc(transformer.DecodeTransaction),
			"span":        DecoderFunc(transformer.DecodeSpan),
			"error":       DecoderFunc(transformer.DecodeError),
			"metricset":   DecoderFunc(transformer.DecodeMetricset),
		},
		MaxEventSize: maxEventSize,
	}
}

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

type Processor struct {
	Decoders      map[string]EventDecoder
	MaxEventSize  int
	bufferPool    sync.Pool
	stoppingError error
	errors        []error
}

const batchSize = 10

func readMetadata(reqMeta map[string]interface{}, reader StreamReader) (*metadata.Metadata, error) {
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

// handleRawModel validates and decodes a single json object into its struct form
func handleRawModel(rawModel map[string]interface{}, decoders map[string]EventDecoder, requestTime time.Time, metadata metadata.Metadata) (publish.Transformable, error) {
	for name, decoder := range decoders {
		if entry, ok := rawModel[name]; ok {
			return decoder.Decode(entry, requestTime, metadata)
		}
	}
	return nil, ErrUnrecognizedObject
}

// readBatch will read up to `batchSize` objects from the ndjson stream
// It returns a slice of transformables and a bool that indicates if there might be more to read.
// It accumulates errors encountered in the processor itself
func (p *Processor) readBatch(ctx context.Context, ipRateLimiter *rate.Limiter, metadata metadata.Metadata, reader StreamReader) ([]publish.Transformable, bool) {
	var (
		err      error
		rawModel map[string]interface{}
		events   []publish.Transformable
	)

	if ipRateLimiter != nil {
		// use provided rate limiter to throttle batch read
		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		err = ipRateLimiter.WaitN(ctxT, batchSize)
		cancel()
		if err != nil {
			p.stoppingError = &Error{
				Type:    RateLimitErrType,
				Message: "rate limit exceeded",
			}
			return events, true
		}
	}

	requestTime := utility.RequestTime(ctx)
	for i := 0; i < batchSize && err == nil; i++ {

		rawModel, err = reader.Read()
		if err != nil && err != io.EOF {

			if e, ok := err.(*Error); ok && (e.Type == InvalidInputErrType || e.Type == InputTooLargeErrType) {
				p.errors = append(p.errors, e)
				continue
			}
			// return early, we assume we can only recover from a input error types
			p.stoppingError = err
			return events, true
		}

		if rawModel != nil {
			evt, err := handleRawModel(rawModel, p.Decoders, requestTime, metadata)
			if err != nil {
				p.errors = append(p.errors, &Error{
					Type:     InvalidInputErrType,
					Message:  err.Error(),
					Document: string(reader.LatestLine()),
				})
				continue
			}
			events = append(events, evt)
		}
	}

	return events, reader.IsEOF()
}

func (p *Processor) HandleStream(ctx context.Context, ipRateLimiter *rate.Limiter, meta map[string]interface{}, reader io.Reader, report publish.Reporter) *Result {
	res := &Result{}
	transformables := p.Process(ctx, ipRateLimiter, meta, reader, report)
	if p.stoppingError != nil {
		res.Add(p.stoppingError)
	}
	for _, err := range p.errors {
		res.LimitedAdd(err)
	}
	res.AddAccepted(len(transformables))
	return res
}

// Process processes a stream of events
// It returns a list of APM Events conforming the Transformable interface
func (p *Processor) Process(ctx context.Context, ipRateLimiter *rate.Limiter, meta map[string]interface{}, reader io.Reader, report publish.Reporter) []publish.Transformable {
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

	metadata, err := readMetadata(meta, jsonReader)
	// no point in continuing if we couldn't read the metadata
	if err != nil {
		p.stoppingError = err
		return nil
	}

	sp, ctx := apm.StartSpan(ctx, "Stream", "Reporter")
	defer sp.End()

	var transformables []publish.Transformable
	for {
		batch, done := p.readBatch(ctx, ipRateLimiter, *metadata, jsonReader)
		if batch != nil {
			err := report(ctx, publish.PendingReq{
				Transformables: batch,
				Trace:          !sp.Dropped(),
			})

			if err != nil {
				switch err {
				case publish.ErrChannelClosed:
					p.stoppingError = &Error{
						Type:    ShuttingDownErrType,
						Message: "server is shutting down",
					}
				case publish.ErrFull:
					p.stoppingError = &Error{
						Type:    QueueFullErrType,
						Message: err.Error(),
					}
				default:
					p.stoppingError = err
				}
				return transformables
			}
			transformables = append(transformables, batch...)
		}

		if done {
			break
		}
	}
	return transformables
}
