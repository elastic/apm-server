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

package model

import (
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/datastreams"
	"github.com/elastic/apm-server/transform"
)

const (
	profileProcessorName = "profile"
	profileDocType       = "profile"
	ProfilesDataset      = "apm.profiling"
)

var profileProcessorEntry = common.MapStr{
	"name":  profileProcessorName,
	"event": profileDocType,
}

// ProfileSample holds a profiling sample.
type ProfileSample struct {
	Metadata  Metadata
	Timestamp time.Time
	Duration  time.Duration
	ProfileID string
	Stack     []ProfileSampleStackframe
	Labels    common.MapStr
	Values    map[string]int64
}

// ProfileSampleStackframe holds details of a stack frame for a profile sample.
type ProfileSampleStackframe struct {
	ID       string
	Function string
	Filename string
	Line     int64
}

func (p *ProfileSample) toBeatEvent(cfg *transform.Config) beat.Event {
	var profileFields mapStr
	profileFields.maybeSetString("id", p.ProfileID)
	if p.Duration > 0 {
		profileFields.set("duration", int64(p.Duration))
	}

	if len(p.Stack) > 0 {
		stackFields := make([]common.MapStr, len(p.Stack))
		for i, frame := range p.Stack {
			frameFields := mapStr{
				"id":       frame.ID,
				"function": frame.Function,
			}
			if frameFields.maybeSetString("filename", frame.Filename) {
				if frame.Line > 0 {
					frameFields.set("line", frame.Line)
				}
			}
			stackFields[i] = common.MapStr(frameFields)
		}
		profileFields.set("stack", stackFields)
		profileFields.set("top", stackFields[0])
	}
	for k, v := range p.Values {
		profileFields.set(k, v)
	}

	fields := mapStr{
		"processor":    profileProcessorEntry,
		profileDocType: common.MapStr(profileFields),
	}
	p.Metadata.set(&fields, p.Labels)
	if cfg.DataStreams {
		fields[datastreams.TypeField] = datastreams.MetricsType
		fields[datastreams.DatasetField] = ProfilesDataset
	}

	return beat.Event{
		Timestamp: p.Timestamp,
		Fields:    common.MapStr(fields),
	}
}
