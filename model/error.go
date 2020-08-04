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
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"strconv"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

var (
	errorMetrics           = monitoring.Default.NewRegistry("apm-server.processor.error")
	errorTransformations   = monitoring.NewInt(errorMetrics, "transformations")
	errorStacktraceCounter = monitoring.NewInt(errorMetrics, "stacktraces")
	errorFrameCounter      = monitoring.NewInt(errorMetrics, "frames")
	errorProcessorEntry    = common.MapStr{"name": errorProcessorName, "event": errorDocType}
)

const (
	errorProcessorName = "error"
	errorDocType       = "error"
)

type Error struct {
	ID            *string
	TransactionID string
	TraceID       string
	ParentID      string

	Timestamp time.Time
	Metadata  Metadata

	Culprit *string
	Labels  *Labels
	Page    *Page
	HTTP    *Http
	URL     *URL
	Custom  *Custom

	Exception *Exception
	Log       *Log

	TransactionSampled *bool
	TransactionType    *string

	// RUM records whether or not this is a RUM error,
	// and should have its stack frames sourcemapped.
	RUM bool

	Experimental interface{}
	data         common.MapStr
}

type Exception struct {
	Message    *string
	Module     *string
	Code       interface{}
	Attributes interface{}
	Stacktrace Stacktrace
	Type       *string
	Handled    *bool
	Cause      []Exception
	Parent     *int
}

type Log struct {
	Message      string
	Level        *string
	ParamMessage *string
	LoggerName   *string
	Stacktrace   Stacktrace
}

func (e *Error) Transform(ctx context.Context, cfg *transform.Config) []beat.Event {
	errorTransformations.Inc()

	if e.Exception != nil {
		addStacktraceCounter(e.Exception.Stacktrace)
	}
	if e.Log != nil {
		addStacktraceCounter(e.Log.Stacktrace)
	}

	fields := common.MapStr{
		"error":     e.fields(ctx, cfg),
		"processor": errorProcessorEntry,
	}

	// first set the generic metadata (order is relevant)
	e.Metadata.Set(fields)
	utility.Set(fields, "source", fields["client"])
	// then add event specific information
	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", e.Labels.Fields())
	utility.Set(fields, "http", e.HTTP.Fields())
	urlFields := e.URL.Fields()
	if urlFields != nil {
		utility.Set(fields, "url", e.URL.Fields())
	}
	if e.Page != nil {
		utility.DeepUpdate(fields, "http.request.referrer", e.Page.Referer)
		if urlFields == nil {
			utility.Set(fields, "url", e.Page.URL.Fields())
		}
	}
	utility.Set(fields, "experimental", e.Experimental)

	// sampled and type is nil if an error happens outside a transaction or an (old) agent is not sending sampled info
	// agents must send semantically correct data
	if e.TransactionSampled != nil || e.TransactionType != nil || e.TransactionID != "" {
		transaction := common.MapStr{}
		if e.TransactionID != "" {
			transaction["id"] = e.TransactionID
		}
		utility.Set(transaction, "type", e.TransactionType)
		utility.Set(transaction, "sampled", e.TransactionSampled)
		utility.Set(fields, "transaction", transaction)
	}

	utility.AddID(fields, "parent", e.ParentID)
	utility.AddID(fields, "trace", e.TraceID)
	utility.Set(fields, "timestamp", utility.TimeAsMicros(e.Timestamp))

	return []beat.Event{
		{
			Fields:    fields,
			Timestamp: e.Timestamp,
		},
	}
}

func (e *Error) fields(ctx context.Context, cfg *transform.Config) common.MapStr {
	e.data = common.MapStr{}
	e.add("id", e.ID)
	e.add("page", e.Page.Fields())

	exceptionChain := flattenExceptionTree(e.Exception)
	e.addException(ctx, cfg, exceptionChain)
	e.addLog(ctx, cfg)

	e.updateCulprit(cfg)
	e.add("culprit", e.Culprit)
	e.add("custom", e.Custom.Fields())

	e.add("grouping_key", e.calcGroupingKey(exceptionChain))

	return e.data
}

func (e *Error) updateCulprit(cfg *transform.Config) {
	if cfg.RUM.SourcemapStore == nil {
		return
	}
	var fr *StacktraceFrame
	if e.Log != nil {
		fr = findSmappedNonLibraryFrame(e.Log.Stacktrace)
	}
	if fr == nil && e.Exception != nil {
		fr = findSmappedNonLibraryFrame(e.Exception.Stacktrace)
	}
	if fr == nil {
		return
	}
	var culprit string
	if fr.Filename != nil {
		culprit = *fr.Filename
	} else if fr.Classname != nil {
		culprit = *fr.Classname
	}
	if fr.Function != nil {
		culprit += fmt.Sprintf(" in %v", *fr.Function)
	}
	e.Culprit = &culprit
}

func findSmappedNonLibraryFrame(frames []*StacktraceFrame) *StacktraceFrame {
	for _, fr := range frames {
		if fr.IsSourcemapApplied() && !fr.IsLibraryFrame() {
			return fr
		}
	}
	return nil
}

func (e *Error) addException(ctx context.Context, cfg *transform.Config, chain []Exception) {
	var result []common.MapStr
	for _, exception := range chain {
		ex := common.MapStr{}
		utility.Set(ex, "message", exception.Message)
		utility.Set(ex, "module", exception.Module)
		utility.Set(ex, "attributes", exception.Attributes)
		utility.Set(ex, "type", exception.Type)
		utility.Set(ex, "handled", exception.Handled)
		utility.Set(ex, "parent", exception.Parent)

		switch code := exception.Code.(type) {
		case int:
			utility.Set(ex, "code", strconv.Itoa(code))
		case float64:
			utility.Set(ex, "code", fmt.Sprintf("%.0f", code))
		case string:
			utility.Set(ex, "code", code)
		case json.Number:
			utility.Set(ex, "code", code.String())
		}

		st := exception.Stacktrace.transform(ctx, cfg, e.RUM, &e.Metadata.Service)
		utility.Set(ex, "stacktrace", st)

		result = append(result, ex)
	}

	e.add("exception", result)
}

func (e *Error) addLog(ctx context.Context, cfg *transform.Config) {
	if e.Log == nil {
		return
	}
	log := common.MapStr{}
	utility.Set(log, "message", e.Log.Message)
	utility.Set(log, "param_message", e.Log.ParamMessage)
	utility.Set(log, "logger_name", e.Log.LoggerName)
	utility.Set(log, "level", e.Log.Level)
	st := e.Log.Stacktrace.transform(ctx, cfg, e.RUM, &e.Metadata.Service)
	utility.Set(log, "stacktrace", st)

	e.add("log", log)
}

type groupingKey struct {
	hash  hash.Hash
	empty bool
}

func newGroupingKey() *groupingKey {
	return &groupingKey{
		hash:  md5.New(),
		empty: true,
	}
}

func (k *groupingKey) add(s *string) bool {
	if s == nil {
		return false
	}
	io.WriteString(k.hash, *s)
	k.empty = false
	return true
}

func (k *groupingKey) addEither(str ...*string) {
	for _, s := range str {
		if ok := k.add(s); ok {
			break
		}
	}
}

func (k *groupingKey) String() string {
	return hex.EncodeToString(k.hash.Sum(nil))
}

// calcGroupingKey computes a value for deduplicating errors - events with
// same grouping key can be collapsed together.
func (e *Error) calcGroupingKey(chain []Exception) string {
	k := newGroupingKey()
	var stacktrace Stacktrace

	for _, ex := range chain {
		k.add(ex.Type)
		stacktrace = append(stacktrace, ex.Stacktrace...)
	}

	if e.Log != nil {
		k.add(e.Log.ParamMessage)
		if len(stacktrace) == 0 {
			stacktrace = e.Log.Stacktrace
		}
	}

	for _, fr := range stacktrace {
		if fr.ExcludeFromGrouping {
			continue
		}
		k.addEither(fr.Module, fr.Filename, fr.Classname)
		k.add(fr.Function)
	}
	if k.empty {
		for _, ex := range chain {
			k.add(ex.Message)
		}
	}
	if k.empty && e.Log != nil {
		k.add(&e.Log.Message)
	}

	return k.String()
}

func (e *Error) add(key string, val interface{}) {
	utility.Set(e.data, key, val)
}

func addStacktraceCounter(st Stacktrace) {
	if frames := len(st); frames > 0 {
		errorStacktraceCounter.Inc()
		errorFrameCounter.Add(int64(frames))
	}
}

// flattenExceptionTree recursively traverses the causes of an exception to return a slice of exceptions.
// Tree traversal is Depth First.
// The parent of a exception in the resulting slice is at the position indicated by the `parent` property
// (0 index based), or the preceding exception if `parent` is nil.
// The resulting exceptions always have `nil` cause.
func flattenExceptionTree(exception *Exception) []Exception {
	var recur func(Exception, int) []Exception

	recur = func(e Exception, posId int) []Exception {
		causes := e.Cause
		e.Cause = nil
		result := []Exception{e}
		for idx, cause := range causes {
			if idx > 0 {
				cause.Parent = &posId
			}
			result = append(result, recur(cause, posId+len(result))...)
		}
		return result
	}

	if exception == nil {
		return []Exception{}
	}
	return recur(*exception, 0)
}
