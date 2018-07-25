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

package span

import (
	"errors"
	"fmt"
	"testing"

	"github.com/elastic/apm-server/model/metadata"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/common"
)

func TestSpanDecode(t *testing.T) {
	id, parent := 1, 12
	name, spType := "foo", "db"
	start, duration := 1.2, 3.4
	context := map[string]interface{}{"a": "b"}
	stacktrace := []interface{}{map[string]interface{}{
		"filename": "file", "lineno": 1.0,
	}}
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *Span
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), s: nil},
		{input: "", err: errors.New("Invalid type for span"), s: nil},
		{
			input: map[string]interface{}{},
			err:   errors.New("Error fetching field"),
			s: &Span{
				Id: nil, Name: "", Type: "", Start: 0.0,
				Duration: 0.0, Context: nil, Parent: nil,
			},
		},
		{
			input: map[string]interface{}{
				"name": name, "id": 1.0, "type": spType,
				"start": start, "duration": duration,
				"context": context, "parent": 12.0,
				"stacktrace": stacktrace,
			},
			err: nil,
			s: &Span{
				Id:       &id,
				Name:     name,
				Type:     spType,
				Start:    start,
				Duration: duration,
				Context:  context,
				Parent:   &parent,
				Stacktrace: m.Stacktrace{
					&m.StacktraceFrame{Filename: "file", Lineno: 1},
				},
			},
		},
	} {
		transformable, err := DecodeSpan(test.input, test.inpErr)

		if test.s != nil {
			span := transformable.(*Span)
			assert.Equal(t, test.s, span)
		} else {
			assert.Nil(t, transformable)
		}
		assert.Equal(t, test.err, err)
	}
}

func TestSpanTransform(t *testing.T) {
	path := "test/path"
	parent := 12
	tid := 1
	service := metadata.Service{Name: "myService"}

	tests := []struct {
		Span   Span
		Output common.MapStr
		Msg    string
	}{
		{
			Span: Span{},
			Output: common.MapStr{
				"type":     "",
				"start":    common.MapStr{"us": 0},
				"duration": common.MapStr{"us": 0},
				"name":     "",
			},
			Msg: "Span without a Stacktrace",
		},
		{
			Span: Span{
				Id:         &tid,
				Name:       "myspan",
				Type:       "myspantype",
				Start:      0.65,
				Duration:   1.20,
				Stacktrace: m.Stacktrace{{AbsPath: &path}},
				Context:    common.MapStr{"key": "val"},
				Parent:     &parent,
			},
			Output: common.MapStr{
				"duration": common.MapStr{"us": 1200},
				"id":       1,
				"name":     "myspan",
				"start":    common.MapStr{"us": 650},
				"type":     "myspantype",
				"parent":   12,
				"stacktrace": []common.MapStr{{
					"exclude_from_grouping": false,
					"abs_path":              path,
					"filename":              "",
					"line":                  common.MapStr{"number": 0},
					"sourcemap": common.MapStr{
						"error":   "Colno mandatory for sourcemapping.",
						"updated": false,
					},
				}},
			},
			Msg: "Full Span",
		},
	}

	tctx := &transform.Context{
		Config: transform.Config{SmapMapper: &sourcemap.SmapMapper{}},
		Metadata: metadata.Metadata{
			Service: &service,
		},
	}
	for idx, test := range tests {
		output := test.Span.Transform(tctx)
		fields := output[0].Fields["span"]
		assert.Equal(t, test.Output, fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))

	}
}
