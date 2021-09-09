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
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/model"
)

func TestBatchProcessor(t *testing.T) {
	client := newMockElasticsearchClient(t, http.StatusOK,
		sourcemapSearchResponseBody(1, []map[string]interface{}{sourcemapHit(string(validSourcemap))}),
	)
	store, err := NewElasticsearchStore(client, "index", time.Minute)
	require.NoError(t, err)

	originalLinenoWithFilename := 1
	originalColnoWithFilename := 7
	originalLinenoWithoutFilename := 1
	originalColnoWithoutFilename := 23
	originalLinenoWithFunction := 1
	originalColnoWithFunction := 67

	nonMatchingFrame := model.StacktraceFrame{
		AbsPath:  "bundle.js",
		Lineno:   newInt(0),
		Colno:    newInt(0),
		Function: "original function",
	}
	mappedFrameWithFilename := model.StacktraceFrame{
		AbsPath:     "bundle.js",
		Function:    "<anonymous>",
		Filename:    "webpack:///bundle.js",
		Lineno:      newInt(1),
		Colno:       newInt(9),
		ContextLine: "/******/ (function(modules) { // webpackBootstrap",
		PostContext: []string{
			"/******/ \t// The module cache",
			"/******/ \tvar installedModules = {};",
			"/******/",
			"/******/ \t// The require function",
			"/******/ \tfunction __webpack_require__(moduleId) {",
		},
		Original: model.Original{
			AbsPath:  "bundle.js",
			Lineno:   &originalLinenoWithFilename,
			Colno:    &originalColnoWithFilename,
			Function: "original function",
		},
		SourcemapUpdated: true,
	}

	mappedFrameWithoutFilename := mappedFrameWithFilename
	mappedFrameWithoutFilename.Original.Lineno = &originalLinenoWithoutFilename
	mappedFrameWithoutFilename.Original.Colno = &originalColnoWithoutFilename
	mappedFrameWithoutFilename.Lineno = newInt(5)
	mappedFrameWithoutFilename.Colno = newInt(0)
	mappedFrameWithoutFilename.Filename = ""
	mappedFrameWithoutFilename.ContextLine = " \tfunction __webpack_require__(moduleId) {"
	mappedFrameWithoutFilename.PreContext = []string{
		" \t// The module cache",
		" \tvar installedModules = {};",
		"",
		" \t// The require function",
	}
	mappedFrameWithoutFilename.PostContext = []string{
		"",
		" \t\t// Check if module is in cache",
		" \t\tif(installedModules[moduleId])",
		" \t\t\treturn installedModules[moduleId].exports;",
		"",
	}

	mappedFrameWithFunction := mappedFrameWithoutFilename
	mappedFrameWithFunction.Original.Lineno = &originalLinenoWithFunction
	mappedFrameWithFunction.Original.Colno = &originalColnoWithFunction
	mappedFrameWithFunction.Lineno = newInt(13)
	mappedFrameWithFunction.Colno = newInt(0)
	mappedFrameWithFunction.ContextLine = " \t\t\texports: {},"
	mappedFrameWithFunction.PreContext = []string{
		" \t\tif(installedModules[moduleId])",
		" \t\t\treturn installedModules[moduleId].exports;",
		"",
		" \t\t// Create a new module (and put it into the cache)",
		" \t\tvar module = installedModules[moduleId] = {",
	}
	mappedFrameWithFunction.PostContext = []string{
		" \t\t\tid: moduleId,",
		" \t\t\tloaded: false",
		" \t\t};",
		"",
		" \t\t// Execute the module function",
	}
	mappedFrameWithFunction2 := mappedFrameWithFunction
	mappedFrameWithFunction2.Function = "exports"

	service := model.Service{
		Name:    "service_name",
		Version: "service_version",
	}

	// Service intentionally left blank
	transaction := model.APMEvent{Transaction: &model.Transaction{}}
	span1 := model.APMEvent{Span: &model.Span{}}

	error1 := model.APMEvent{
		Service: service,
		Error:   &model.Error{},
	}
	span2 := model.APMEvent{
		Service: service,
		Span: &model.Span{
			Stacktrace: model.Stacktrace{cloneFrame(nonMatchingFrame), {
				AbsPath:  "bundle.js",
				Lineno:   newInt(originalLinenoWithFilename),
				Colno:    newInt(originalColnoWithFilename),
				Function: "original function",
			}},
		},
	}
	error2 := model.APMEvent{
		Service: service,
		Error: &model.Error{
			Log: &model.Log{
				Stacktrace: model.Stacktrace{{
					AbsPath:  "bundle.js",
					Lineno:   newInt(originalLinenoWithoutFilename),
					Colno:    newInt(originalColnoWithoutFilename),
					Function: "original function",
				}},
			},
		},
	}
	error3 := model.APMEvent{
		Service: service,
		Error: &model.Error{
			Exception: &model.Exception{
				Stacktrace: model.Stacktrace{{
					AbsPath:  "bundle.js",
					Lineno:   newInt(originalLinenoWithFunction),
					Colno:    newInt(originalColnoWithFunction),
					Function: "original function",
				}},
				Cause: []model.Exception{{
					Stacktrace: model.Stacktrace{{
						AbsPath:  "bundle.js",
						Lineno:   newInt(originalLinenoWithFunction),
						Colno:    newInt(originalColnoWithFunction),
						Function: "original function",
					}, {
						AbsPath:  "bundle.js",
						Lineno:   newInt(originalLinenoWithFunction),
						Colno:    newInt(originalColnoWithFunction),
						Function: "original function",
					}},
				}},
			},
		},
	}

	processor := BatchProcessor{Store: store}
	err = processor.ProcessBatch(context.Background(), &model.Batch{transaction, span1, span2, error1, error2, error3})
	assert.NoError(t, err)

	assert.Equal(t, &model.Span{}, span1.Span)
	assert.Equal(t, &model.Error{}, error1.Error)
	assert.Equal(t, &model.Span{
		Stacktrace: model.Stacktrace{
			cloneFrame(nonMatchingFrame),
			cloneFrame(mappedFrameWithFilename),
		},
	}, span2.Span)
	assert.Equal(t, &model.Error{
		Log: &model.Log{
			Stacktrace: model.Stacktrace{
				cloneFrame(mappedFrameWithoutFilename),
			},
		},
	}, error2.Error)
	assert.Equal(t, &model.Error{
		Exception: &model.Exception{
			Stacktrace: model.Stacktrace{
				cloneFrame(mappedFrameWithFunction),
			},
			Cause: []model.Exception{{
				Stacktrace: model.Stacktrace{
					cloneFrame(mappedFrameWithFunction2),
					cloneFrame(mappedFrameWithFunction),
				},
			}},
		},
	}, error3.Error)
}

func TestBatchProcessorElasticsearchUnavailable(t *testing.T) {
	client := newUnavailableElasticsearchClient(t)
	store, err := NewElasticsearchStore(client, "index", time.Minute)
	require.NoError(t, err)

	nonMatchingFrame := model.StacktraceFrame{
		AbsPath:  "bundle.js",
		Lineno:   newInt(0),
		Colno:    newInt(0),
		Function: "original function",
	}

	span := model.APMEvent{
		Service: model.Service{
			Name:    "service_name",
			Version: "service_version",
		},
		Span: &model.Span{
			Stacktrace: model.Stacktrace{cloneFrame(nonMatchingFrame), cloneFrame(nonMatchingFrame)},
		},
	}

	logp.DevelopmentSetup(logp.ToObserverOutput())
	for i := 0; i < 2; i++ {
		processor := BatchProcessor{Store: store}
		err = processor.ProcessBatch(context.Background(), &model.Batch{span, span})
		assert.NoError(t, err)
	}

	// SourcemapError should have been set, but the frames should otherwise be unmodified.
	expectedFrame := nonMatchingFrame
	expectedFrame.SourcemapError = "failure querying ES: client error"
	assert.Equal(t, model.Stacktrace{&expectedFrame, &expectedFrame}, span.Span.Stacktrace)

	// We should have a single log message, due to rate limiting.
	entries := logp.ObserverLogs().TakeAll()
	require.Len(t, entries, 1)
	assert.Equal(t, "failed to fetch source map: failure querying ES: client error", entries[0].Message)
}

func TestBatchProcessorTimeout(t *testing.T) {
	var transport roundTripperFunc = func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}

	cfg := elasticsearch.DefaultConfig()
	cfg.Hosts = []string{""}
	client, err := elasticsearch.NewClientParams(elasticsearch.ClientParams{
		Config:    cfg,
		Transport: transport,
	})
	require.NoError(t, err)
	store, err := NewElasticsearchStore(client, "index", time.Minute)
	require.NoError(t, err)

	frame := model.StacktraceFrame{
		AbsPath:  "bundle.js",
		Lineno:   newInt(0),
		Colno:    newInt(0),
		Function: "original function",
	}
	span := model.APMEvent{
		Service: model.Service{
			Name:    "service_name",
			Version: "service_version",
		},
		Span: &model.Span{
			Stacktrace: model.Stacktrace{cloneFrame(frame)},
		},
	}

	before := time.Now()
	processor := BatchProcessor{Store: store, Timeout: 100 * time.Millisecond}
	err = processor.ProcessBatch(context.Background(), &model.Batch{span})
	assert.NoError(t, err)
	taken := time.Since(before)
	assert.Less(t, taken, time.Second)
}

func cloneFrame(frame model.StacktraceFrame) *model.StacktraceFrame {
	return &frame
}

func newInt(v int) *int {
	return &v
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
