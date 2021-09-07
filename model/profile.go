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

	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	ProfilesDataset = "apm.profiling"
)

// ProfileProcessor is the Processor value that should be assigned to profile events.
var ProfileProcessor = Processor{Name: "profile", Event: "profile"}

// ProfileSample holds a profiling sample.
type ProfileSample struct {
	Duration  time.Duration
	ProfileID string
	Stack     []ProfileSampleStackframe
	Values    map[string]int64
}

// ProfileSampleStackframe holds details of a stack frame for a profile sample.
type ProfileSampleStackframe struct {
	ID       string
	Function string
	Filename string
	Line     int64
}

func (p *ProfileSample) setFields(fields *mapStr) {
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
	fields.set("profile", common.MapStr(profileFields))
}
