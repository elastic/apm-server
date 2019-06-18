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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/beat"
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

func (v *intakeTestProcessor) getReader(path string) (*decoder.NDJSONStreamReader, error) {
	reader, err := loader.LoadDataAsStream(path)
	if err != nil {
		return nil, err
	}
	lr := decoder.NewLineReader(bufio.NewReaderSize(reader, lrSize), lrSize)
	return decoder.NewNDJSONStreamReader(lr), nil
}

func (v *intakeTestProcessor) readEvents(reader *decoder.NDJSONStreamReader) ([]interface{}, error) {
	var (
		err    error
		e      map[string]interface{}
		events []interface{}
	)

	for err != io.EOF {
		e, err, _ = reader.Read()
		if err != nil && err != io.EOF {
			return events, err
		}
		if e != nil {
			events = append(events, e)
		}
	}
	return events, nil
}

func (p *intakeTestProcessor) LoadPayload(path string) (interface{}, error) {
	ndjson, err := p.getReader(path)
	if err != nil {
		return nil, err
	}

	// read and discard metadata
	ndjson.Read()

	return p.readEvents(ndjson)
}

func (p *intakeTestProcessor) Decode(data interface{}) error {
	events := data.([]interface{})
	for _, e := range events {
		_, err := p.Processor.HandleRawModel(e.(map[string]interface{}))
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

	result := p.HandleStream(context.TODO(), nil, nil, bytes.NewBuffer(buf), report)
	var events []beat.Event
	for _, req := range reqs {
		if req.Transformables != nil {
			for _, transformable := range req.Transformables {
				events = append(events, transformable.Transform(req.Tcontext)...)
			}
		}
	}

	if result.IsError() {
		return events, errors.New(fmt.Sprintf("+%v", result.Body()))
	}

	return events, nil
}
