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

func metadataFields() *tests.Set {
	return tests.NewSet(
		// metadata fields
		"context.system",
		"context.service.language.name",
		"context.system.architecture",
		"context.service.language.version",
		"context.service.framework.name",
		"context.service.environment",
		"context.service.framework.version",
		"context.process.title",
		"context.service.name",
		"context.system.platform",
		"context.system.hostname",
		"context.service.version",
		"context.service.runtime.name",
		"context.service.agent.name",
		"context.service.agent.version",
		"context.service.runtime.version",
	)

}

type V2TestProcessor struct {
	stream.StreamProcessor
}

func (p *V2TestProcessor) LoadPayload(path string) (interface{}, error) {
	reader, err := loader.LoadDataAsStream(path)
	if err != nil {
		return nil, err
	}
	ndjson := decoder.NewNDJSONStreamReader(reader, 100*1024)

	// read and discard metadata
	ndjson.Read()

	var e map[string]interface{}
	var events []interface{}
	for err != io.EOF {
		e, err = ndjson.Read()
		if err != nil && err != io.EOF {
			return events, err
		}
		if e != nil {
			events = append(events, e)
		}
	}
	return events, nil
}

func (p *V2TestProcessor) Decode(data interface{}) error {
	events := data.([]interface{})
	for _, e := range events {
		_, err := p.StreamProcessor.HandleRawModel(e.(map[string]interface{}))
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *V2TestProcessor) Validate(data interface{}) error {
	events := data.([]interface{})
	for _, e := range events {
		_, err := p.StreamProcessor.HandleRawModel(e.(map[string]interface{}))
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *V2TestProcessor) Process(buf []byte) ([]beat.Event, error) {
	ndjsonreader := decoder.NewNDJSONStreamReader(bytes.NewBuffer(buf), 100*1024)

	var reqs []publish.PendingReq
	report := tests.TestReporter(&reqs)

	result := p.HandleStream(context.TODO(), nil, ndjsonreader, report)
	var events []beat.Event
	for _, req := range reqs {
		if req.Transformables != nil {
			for _, transformable := range req.Transformables {
				events = append(events, transformable.Transform(req.Tcontext)...)
			}
		}
	}

	if len(result.Errors) > 0 {
		return events, errors.New(result.String())
	}

	return events, nil
}
