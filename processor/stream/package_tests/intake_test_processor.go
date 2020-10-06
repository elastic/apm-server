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

package package_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder"
	v2 "github.com/elastic/apm-server/model/modeldecoder/v2"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
)

type intakeTestProcessor struct {
	stream.Processor
}

const lrSize = 100 * 1024

func (v *intakeTestProcessor) getDecoder(path string) (*decoder.NDJSONStreamDecoder, error) {
	reader, err := loader.LoadDataAsStream(path)
	if err != nil {
		return nil, err
	}
	return decoder.NewNDJSONStreamDecoder(reader, lrSize), nil
}

func (v *intakeTestProcessor) readEvents(dec *decoder.NDJSONStreamDecoder) ([]interface{}, error) {
	var (
		err    error
		events []interface{}
	)

	for err != io.EOF {
		var e map[string]interface{}
		if err = dec.Decode(&e); err != nil && err != io.EOF {
			return events, err
		}
		if e != nil {
			events = append(events, e)
		}
	}
	return events, nil
}

func (p *intakeTestProcessor) LoadPayload(path string) (interface{}, error) {
	ndjson, err := p.getDecoder(path)
	if err != nil {
		return nil, err
	}

	// read and discard metadata
	var m map[string]interface{}
	ndjson.Decode(&m)

	return p.readEvents(ndjson)
}

func (p *intakeTestProcessor) Decode(data interface{}) error {
	events := data.([]interface{})
	for _, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		d := decoder.NewNDJSONStreamDecoder(bytes.NewReader(b), 300*1024)
		body, err := d.ReadAhead()
		if err != nil && err != io.EOF {
			return err
		}
		eventType := p.IdentifyEventType(body, &stream.Result{})
		input := modeldecoder.Input{
			RequestTime: time.Now(),
			Metadata:    model.Metadata{},
			Config:      p.Mconfig,
		}
		switch eventType {
		case "error":
			var event model.Error
			err = v2.DecodeNestedError(d, &input, &event)
		case "span":
			var event model.Span
			err = v2.DecodeNestedSpan(d, &input, &event)
		case "transaction":
			var event model.Transaction
			err = v2.DecodeNestedTransaction(d, &input, &event)
		case "metricset":
			var m map[string]interface{}
			if err = d.Decode(&m); err != nil && err != io.EOF {
				return err
			}
			input.Raw = m[eventType]
			err = modeldecoder.DecodeMetricset(input, &model.Batch{})
		default:
			return errors.New("root key required")
		}
		if err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}

func (p *intakeTestProcessor) Validate(data interface{}) error {
	return p.Decode(data)
}

func (p *intakeTestProcessor) Process(buf []byte) ([]beat.Event, error) {
	var reqs []publish.PendingReq
	report := tests.TestReporter(&reqs)

	result := p.HandleStream(context.TODO(), nil, &model.Metadata{}, bytes.NewBuffer(buf), report)
	var events []beat.Event
	for _, req := range reqs {
		if req.Transformables != nil {
			for _, transformable := range req.Transformables {
				events = append(events, transformable.Transform(context.Background(), &transform.Config{})...)
			}
		}
	}

	if len(result.Errors) > 0 {
		return events, errors.New(result.Error())
	}

	return events, nil
}
