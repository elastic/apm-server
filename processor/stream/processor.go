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
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

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
	"github.com/elastic/apm-server/server"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	ErrUnrecognizedObject = errors.New("did not recognize object type")
)

type StreamReader interface {
	Read() (map[string]interface{}, error, bool)
	IsEOF() bool
	LatestLine() []byte
}

type Processor struct {
	Tconfig      transform.Config
	Mconfig      model.Config
	MaxEventSize int
	bufferPool   sync.Pool
}

const batchSize = 10

var models = []struct {
	key          string
	schema       *jsonschema.Schema
	modelDecoder func(interface{}, model.Config, error) (transform.Transformable, error)
}{
	{
		"transaction",
		transaction.ModelSchema(),
		transaction.DecodeEvent,
	},
	{
		"span",
		span.ModelSchema(),
		span.DecodeEvent,
	},
	{
		"metricset",
		metricset.ModelSchema(),
		metricset.DecodeEvent,
	},
	{
		"error",
		er.ModelSchema(),
		er.DecodeEvent,
	},
}

func (p *Processor) readMetadata(reqMeta map[string]interface{}, reader StreamReader) (*metadata.Metadata, result) {

	// first item is the metadata object
	rawModel, err, _ := reader.Read()
	if err != nil {
		return nil, newErroredResult(enrich(err, reader.LatestLine()))
	}

	rawMetadata, ok := rawModel["metadata"].(map[string]interface{})
	if !ok {
		return nil, newErroredResult(enrich(ErrUnrecognizedObject, reader.LatestLine()))
	}

	for k, v := range reqMeta {
		utility.InsertInMap(rawMetadata, k, v.(map[string]interface{}))
	}

	// validate the metadata object against our jsonschema
	err = validation.Validate(rawMetadata, metadata.ModelSchema())
	if err != nil {
		return nil, newErroredResult(enrich(err, reader.LatestLine()))
	}

	// create a metadata struct
	metadata, err := metadata.DecodeMetadata(rawMetadata)
	if err != nil {
		return nil, newErroredResult(enrich(err, reader.LatestLine()))
	}

	return metadata, result{}
}

// HandleRawModel validates and decodes a single json object into its struct form
func (p *Processor) HandleRawModel(rawModel map[string]interface{}) (transform.Transformable, error) {
	for _, model := range models {
		if entry, ok := rawModel[model.key]; ok {
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
func (p *Processor) readBatch(ctx context.Context, rl *rate.Limiter, batchSize int, reader StreamReader, res result) ([]transform.Transformable, bool, result) {
	var (
		err        error
		eventables []transform.Transformable
	)
	if rl != nil {
		// use provided rate limiter to throttle batch read
		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		err = rl.WaitN(ctxT, batchSize)
		cancel()
		if err != nil {
			res.add(server.RateLimited())
			return eventables, true, res
		}
	}

	for i := 0; i < batchSize && err == nil; i++ {

		rawModel, err, stop := reader.Read()
		if err != nil && err != io.EOF {
			res.add(&server.Error{enrich(err, reader.LatestLine()), http.StatusBadRequest})
			if stop {
				return eventables, true, res
			}
		} else if rawModel != nil {
			tr, err := p.HandleRawModel(rawModel)
			if err != nil {
				res.add(&server.Error{enrich(err, reader.LatestLine()), http.StatusBadRequest})
			} else {
				eventables = append(eventables, tr)
			}
		}
	}

	return eventables, reader.IsEOF(), res
}

func enrich(e error, b []byte) error {
	if s := string(b); s != "" && e != nil {
		return errors.New(fmt.Sprintf("%s [%s]", e.Error(), s))
	}
	return e
}

func (p *Processor) HandleStream(ctx context.Context, rl *rate.Limiter, meta map[string]interface{}, reader io.Reader, report publish.Reporter) server.Response {

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

	metadata, result := p.readMetadata(meta, ndReader)
	// no point in continuing if we couldn't read the metadata
	if result.IsError() {
		return result
	}

	tctx := &transform.Context{
		RequestTime: utility.RequestTime(ctx),
		Config:      p.Tconfig,
		Metadata:    *metadata,
	}

	sp, ctx := apm.StartSpan(ctx, "Stream", "Reporter")
	defer sp.End()

	for {

		var transformables []transform.Transformable
		var done bool
		transformables, done, result = p.readBatch(ctx, rl, batchSize, ndReader, result)
		if transformables != nil {
			err := report(ctx, publish.PendingReq{
				Transformables: transformables,
				Tcontext:       tctx,
				Trace:          !sp.Dropped(),
			})

			if err != nil {
				switch err {
				case publish.ErrChannelClosed:
					result.add(server.ShuttingDown())
				case publish.ErrFull:
					result.add(&server.Error{err, http.StatusServiceUnavailable})
				default:
					result.add(&server.Error{err, http.StatusInternalServerError})
				}
				return result
			}
			result.addAccepted(len(transformables))
		}

		if done {
			break
		}
	}
	return result
}
