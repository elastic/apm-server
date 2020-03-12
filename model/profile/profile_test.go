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

package profile_test

import (
	"testing"
	"time"

	pprof_profile "github.com/google/pprof/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/profile"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestPprofProfileTransform(t *testing.T) {
	timestamp := time.Unix(123, 456)
	pp := profile.PprofProfile{
		Profile: &pprof_profile.Profile{
			TimeNanos:     timestamp.UnixNano(),
			DurationNanos: int64(10 * time.Second),
			SampleType: []*pprof_profile.ValueType{
				{Type: "cpu", Unit: "nanoseconds"},
				{Type: "inuse_space", Unit: "bytes"},
			},
			Sample: []*pprof_profile.Sample{{
				Value: []int64{123, 456},
				Label: map[string][]string{
					"key1": []string{"abc", "def"},
					"key2": []string{"ghi"},
				},
				Location: []*pprof_profile.Location{{
					Line: []pprof_profile.Line{{
						Function: &pprof_profile.Function{Name: "foo", Filename: "foo.go"},
						Line:     1,
					}},
				}, {
					Line: []pprof_profile.Line{{
						Function: &pprof_profile.Function{Name: "bar", Filename: "bar.go"},
					}},
				}},
			}, {
				Value: []int64{123, 456},
				Label: map[string][]string{
					"key1": []string{"abc", "def"},
					"key2": []string{"ghi"},
				},
				Location: []*pprof_profile.Location{{
					Line: []pprof_profile.Line{{
						Function: &pprof_profile.Function{Name: "foo", Filename: "foo.go"},
						Line:     1,
					}},
				}, {
					Line: []pprof_profile.Line{{
						Function: &pprof_profile.Function{Name: "bar", Filename: "bar.go"},
					}},
				}},
			}},
		},
	}

	serviceName, env := "myService", "staging"
	service := metadata.Service{
		Name:        &serviceName,
		Environment: &env,
	}
	metadata := metadata.Metadata{Service: &service}

	tctx := &transform.Context{
		Config:      transform.Config{SourcemapStore: &sourcemap.Store{}},
		Metadata:    metadata,
		RequestTime: time.Time{}, // not used
	}
	output := pp.Transform(tctx)
	require.Len(t, output, 2)
	assert.Equal(t, output[0], output[1])

	assert.Equal(t, beat.Event{
		Timestamp: timestamp,
		Fields: common.MapStr{
			"processor": common.MapStr{"event": "profile", "name": "profile"},
			"service": common.MapStr{
				"name":        "myService",
				"environment": "staging",
			},
			"labels": common.MapStr{
				"key1": []string{"abc", "def"},
				"key2": []string{"ghi"},
			},
			"profile": common.MapStr{
				"duration":          int64(10 * time.Second),
				"cpu.ns":            int64(123),
				"inuse_space.bytes": int64(456),
				"top": common.MapStr{
					"function": "foo",
					"filename": "foo.go",
					"line":     int64(1),
					"id":       "98430081820ed765",
				},
				"stack": []common.MapStr{{
					"function": "foo",
					"filename": "foo.go",
					"line":     int64(1),
					"id":       "98430081820ed765",
				}, {
					"function": "bar",
					"filename": "bar.go",
					"id":       "48a37c90ad27a659",
				}},
			},
		},
	}, output[0])
}
