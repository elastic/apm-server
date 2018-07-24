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

package sourcemap

import (
	"errors"
	"fmt"

	parser "github.com/go-sourcemap/sourcemap"

	sm "github.com/elastic/apm-server/model/sourcemap"
	"github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	Processor = &sourcemapProcessor{
		processor.PayloadProcessor{
			ProcessorName: "sourcemap",
			DecodePayload: sm.DecodePayload,
			PayloadSchema: sm.PayloadSchema(),
			DecodingCount: monitoring.NewInt(sm.Metrics, "decoding.count"),
			DecodingError: monitoring.NewInt(sm.Metrics, "decoding.errors"),
			ValidateCount: monitoring.NewInt(sm.Metrics, "validation.count"),
			ValidateError: monitoring.NewInt(sm.Metrics, "validation.errors"),
		},
	}
)

type sourcemapProcessor struct {
	processor.PayloadProcessor
}

func (p *sourcemapProcessor) Validate(raw map[string]interface{}) error {
	p.PayloadProcessor.ValidateCount.Inc()

	smap, ok := raw["sourcemap"].(string)
	if !ok {
		if s, _ := raw["sourcemap"]; s == nil {
			return errors.New(`missing properties: "sourcemap", expected sourcemap to be sent as string, but got null`)
		} else {
			return errors.New("sourcemap not in expected format")
		}
	}

	_, err := parser.Parse("", []byte(smap))
	if err != nil {
		return errors.New(fmt.Sprintf("Error validating sourcemap: %v", err))
	}

	err = validation.Validate(raw, p.PayloadSchema)
	if err != nil {
		p.PayloadProcessor.ValidateError.Inc()
	}
	return err
}
