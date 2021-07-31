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

package model_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestProfileSampleTransform(t *testing.T) {
	timestamp := time.Unix(123, 456)
	sample := model.ProfileSample{
		Duration:  10 * time.Second,
		ProfileID: "profile_id",
		Stack: []model.ProfileSampleStackframe{{
			ID:       "foo_id",
			Function: "foo",
			Filename: "foo.go",
			Line:     1,
		}, {
			ID:       "bar_id",
			Function: "bar",
			Filename: "bar.go",
		}},
		Values: map[string]int64{
			"samples.count":     1,
			"cpu.ns":            123,
			"wall.us":           789,
			"inuse_space.bytes": 456,
		},
	}

	batch := &model.Batch{{
		Timestamp:     timestamp,
		ProfileSample: &sample,
	}, {
		Timestamp:     timestamp,
		ProfileSample: &sample,
	}}
	output := batch.Transform(context.Background())
	require.Len(t, output, 2)
	assert.Equal(t, output[0], output[1])

	assert.Equal(t, beat.Event{
		Timestamp: timestamp,
		Fields: common.MapStr{
			"processor": common.MapStr{"event": "profile", "name": "profile"},
			"profile": common.MapStr{
				"id":                "profile_id",
				"duration":          int64(10 * time.Second),
				"cpu.ns":            int64(123),
				"wall.us":           int64(789),
				"inuse_space.bytes": int64(456),
				"samples.count":     int64(1),
				"top": common.MapStr{
					"function": "foo",
					"filename": "foo.go",
					"line":     int64(1),
					"id":       "foo_id",
				},
				"stack": []common.MapStr{{
					"function": "foo",
					"filename": "foo.go",
					"line":     int64(1),
					"id":       "foo_id",
				}, {
					"function": "bar",
					"filename": "bar.go",
					"id":       "bar_id",
				}},
			},
		},
	}, output[0])
}
