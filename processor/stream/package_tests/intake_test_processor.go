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
	"errors"
	"io"
	"time"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/v7/libbeat/beat"
)

type TestSetup struct {
	InputDataPath string
	TemplatePaths []string
	Schema        *jsonschema.Schema
}

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
		err := p.Processor.HandleRawModel(e.(map[string]interface{}), &model.Batch{}, time.Now(), model.Metadata{})
		if err != nil {
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
