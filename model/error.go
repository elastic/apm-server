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

	"github.com/elastic/apm-server/datastreams"
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
	ErrorsDataset      = "apm.error"
)

type Error struct {
	ID            string
	TransactionID string
	TraceID       string
	ParentID      string

	Timestamp time.Time
	Metadata  Metadata

	Culprit string
	Labels  common.MapStr
	Page    *Page
	HTTP    *Http
	URL     *URL
	Custom  common.MapStr

	Exception *Exception
	Log       *Log

	TransactionSampled *bool
	TransactionType    string

	// RUM records whether or not this is a RUM error,
	// and should have its stack frames sourcemapped.
	RUM bool

	Experimental interface{}
}

type Exception struct {
	Message    string
	Module     string
	Code       interface{}
	Attributes interface{}
	Stacktrace Stacktrace
	Type       string
	Handled    *bool
	Cause      []Exception
	Parent     *int
}

type Log struct {
	Message      string
	Level        string
	ParamMessage string
	LoggerName   string
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

	if cfg.DataStreams {
		// Errors are stored in an APM errors-specific "logs" data stream, per service.
		// By storing errors in a "logs" data stream, they can be viewed in the Logs app
		// in Kibana.
		fields[datastreams.TypeField] = datastreams.LogsType
		dataset := fmt.Sprintf("%s.%s", ErrorsDataset, datastreams.NormalizeServiceName(e.Metadata.Service.Name))
		fields[datastreams.DatasetField] = dataset
	}

	// first set the generic metadata (order is relevant)
	e.Metadata.Set(fields, e.Labels)
	utility.Set(fields, "source", fields["client"])
	// then add event specific information
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
	var transaction mapStr
	transaction.maybeSetString("id", e.TransactionID)
	transaction.maybeSetString("type", e.TransactionType)
	transaction.maybeSetBool("sampled", e.TransactionSampled)
	utility.Set(fields, "transaction", common.MapStr(transaction))

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
	var fields mapStr
	fields.maybeSetString("id", e.ID)
	fields.maybeSetMapStr("page", e.Page.Fields())

	exceptionChain := flattenExceptionTree(e.Exception)
	if exception := e.exceptionFields(ctx, cfg, exceptionChain); len(exception) > 0 {
		fields.set("exception", exception)
	}
	fields.maybeSetMapStr("log", e.logFields(ctx, cfg))

	e.updateCulprit(cfg)
	fields.maybeSetString("culprit", e.Culprit)
	fields.maybeSetMapStr("custom", customFields(e.Custom))
	fields.maybeSetString("grouping_key", e.calcGroupingKey(exceptionChain))
	return common.MapStr(fields)
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
	if fr.Filename != "" {
		culprit = fr.Filename
	} else if fr.Classname != "" {
		culprit = fr.Classname
	}
	if fr.Function != "" {
		culprit += fmt.Sprintf(" in %v", fr.Function)
	}
	e.Culprit = culprit
}

func findSmappedNonLibraryFrame(frames []*StacktraceFrame) *StacktraceFrame {
	for _, fr := range frames {
		if fr.IsSourcemapApplied() && !fr.IsLibraryFrame() {
			return fr
		}
	}
	return nil
}

func (e *Error) exceptionFields(ctx context.Context, cfg *transform.Config, chain []Exception) []common.MapStr {
	var result []common.MapStr
	for _, exception := range chain {
		var ex mapStr
		ex.maybeSetString("message", exception.Message)
		ex.maybeSetString("module", exception.Module)
		ex.maybeSetString("type", exception.Type)
		ex.maybeSetBool("handled", exception.Handled)
		if exception.Parent != nil {
			ex.set("parent", exception.Parent)
		}
		if exception.Attributes != nil {
			ex.set("attributes", exception.Attributes)
		}

		switch code := exception.Code.(type) {
		case int:
			ex.set("code", strconv.Itoa(code))
		case float64:
			ex.set("code", fmt.Sprintf("%.0f", code))
		case string:
			ex.set("code", code)
		case json.Number:
			ex.set("code", code.String())
		}

		if st := exception.Stacktrace.transform(ctx, cfg, e.RUM, &e.Metadata.Service); len(st) > 0 {
			ex.set("stacktrace", st)
		}

		result = append(result, common.MapStr(ex))
	}
	return result
}

func (e *Error) logFields(ctx context.Context, cfg *transform.Config) common.MapStr {
	if e.Log == nil {
		return nil
	}
	var log mapStr
	log.maybeSetString("message", e.Log.Message)
	log.maybeSetString("param_message", e.Log.ParamMessage)
	log.maybeSetString("logger_name", e.Log.LoggerName)
	log.maybeSetString("level", e.Log.Level)
	if st := e.Log.Stacktrace.transform(ctx, cfg, e.RUM, &e.Metadata.Service); len(st) > 0 {
		log.set("stacktrace", st)
	}
	return common.MapStr(log)
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

func (k *groupingKey) add(s string) bool {
	if s == "" {
		return false
	}
	io.WriteString(k.hash, s)
	k.empty = false
	return true
}

func (k *groupingKey) addEither(str ...string) {
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
		k.add(e.Log.Message)
	}

	return k.String()
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
