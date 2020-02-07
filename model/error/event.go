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

package error

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/elastic/apm-server/sourcemap"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/error/generated/schema"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	Metrics           = monitoring.Default.NewRegistry("apm-server.processor.error", monitoring.PublishExpvar)
	transformations   = monitoring.NewInt(Metrics, "transformations")
	stacktraceCounter = monitoring.NewInt(Metrics, "stacktraces")
	frameCounter      = monitoring.NewInt(Metrics, "frames")
	processorEntry    = common.MapStr{"name": processorName, "event": errorDocType}
	errInvalidType    = errors.New("invalid type for error event")
)

const (
	processorName = "error"
	errorDocType  = "error"
	emptyString   = ""
)

var cachedModelSchema = validation.CreateSchema(schema.ModelSchema, processorName)

type Event struct {
	Id            *string
	TransactionId *string
	TraceId       *string
	ParentId      *string

	Timestamp time.Time

	Culprit *string
	User    *metadata.User
	Labels  *m.Labels
	Page    *m.Page
	Http    *m.Http
	Url     *m.Url
	Custom  *m.Custom
	Service *metadata.Service
	Client  *m.Client

	Exception *Exception
	Log       *Log

	TransactionSampled *bool
	TransactionType    *string

	Experimental interface{}
	data         common.MapStr

	Metadata metadata.Metadata
}

type Exception struct {
	Message    *string
	Module     *string
	Code       interface{}
	Attributes interface{}
	Stacktrace m.Stacktrace
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
	Stacktrace   m.Stacktrace
}

func Decode(input interface{}, requestTime time.Time, metadata metadata.Metadata, experimental bool) (*Event, error) {
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errInvalidType
	}
	err := validation.Validate(input, cachedModelSchema)
	if err != nil {
		return nil, err
	}

	ctx, err := m.DecodeContext(raw, experimental)
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	e := Event{
		Id:                 decoder.StringPtr(raw, "id"),
		Culprit:            decoder.StringPtr(raw, "culprit"),
		Labels:             ctx.Labels,
		Page:               ctx.Page,
		Http:               ctx.Http,
		Url:                ctx.Url,
		Custom:             ctx.Custom,
		User:               ctx.User,
		Service:            ctx.Service,
		Experimental:       ctx.Experimental,
		Client:             ctx.Client,
		Timestamp:          decoder.TimeEpochMicro(raw, "timestamp", requestTime),
		TransactionId:      decoder.StringPtr(raw, "transaction_id"),
		ParentId:           decoder.StringPtr(raw, "parent_id"),
		TraceId:            decoder.StringPtr(raw, "trace_id"),
		TransactionSampled: decoder.BoolPtr(raw, "sampled", "transaction"),
		TransactionType:    decoder.StringPtr(raw, "type", "transaction"),
		Metadata:           metadata,
	}

	ex := decoder.MapStr(raw, "exception")
	e.Exception = decodeException(&decoder)(ex)

	log := decoder.MapStr(raw, "log")
	logMsg := decoder.StringPtr(log, "message")
	if logMsg != nil {
		e.Log = &Log{
			Message:      *logMsg,
			ParamMessage: decoder.StringPtr(log, "param_message"),
			Level:        decoder.StringPtr(log, "level"),
			LoggerName:   decoder.StringPtr(log, "logger_name"),
			Stacktrace:   m.Stacktrace{},
		}
		var stacktrace *m.Stacktrace
		stacktrace, decoder.Err = m.DecodeStacktrace(log["stacktrace"], decoder.Err)
		if stacktrace != nil {
			e.Log.Stacktrace = *stacktrace
		}
	}
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	return &e, nil
}

func (e *Event) Transform(libraryPattern, excludeFromGrouping *regexp.Regexp, sourcemapStore *sourcemap.Store) []beat.Event {
	transformations.Inc()

	if e.Exception != nil {
		addStacktraceCounter(e.Exception.Stacktrace)
	}
	if e.Log != nil {
		addStacktraceCounter(e.Log.Stacktrace)
	}

	fields := common.MapStr{
		"error":     e.fields(libraryPattern, excludeFromGrouping, sourcemapStore),
		"processor": processorEntry,
	}

	// first set the generic metadata (order is relevant)
	e.Metadata.Set(fields)
	// then add event specific information
	utility.Update(fields, "user", e.User.Fields())
	clientFields := e.Client.Fields()
	utility.DeepUpdate(fields, "client", clientFields)
	utility.DeepUpdate(fields, "source", clientFields)
	utility.DeepUpdate(fields, "user_agent", e.User.UserAgentFields())
	utility.DeepUpdate(fields, "service", e.Service.Fields(emptyString, emptyString))
	utility.DeepUpdate(fields, "agent", e.Service.AgentFields())
	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", e.Labels.Fields())
	utility.Set(fields, "http", e.Http.Fields())
	utility.Set(fields, "url", e.Url.Fields())
	utility.Set(fields, "experimental", e.Experimental)

	// sampled and type is nil if an error happens outside a transaction or an (old) agent is not sending sampled info
	// agents must send semantically correct data
	if e.TransactionSampled != nil || e.TransactionType != nil || (e.TransactionId != nil && *e.TransactionId != "") {
		transaction := common.MapStr{}
		utility.Set(transaction, "id", e.TransactionId)
		utility.Set(transaction, "type", e.TransactionType)
		utility.Set(transaction, "sampled", e.TransactionSampled)
		utility.Set(fields, "transaction", transaction)
	}

	utility.AddId(fields, "parent", e.ParentId)
	utility.AddId(fields, "trace", e.TraceId)

	utility.Set(fields, "timestamp", utility.TimeAsMicros(e.Timestamp))

	return []beat.Event{
		{
			Fields:    fields,
			Timestamp: e.Timestamp,
		},
	}
}

func (e *Event) fields(libraryPattern, excludeFromGrouping *regexp.Regexp, sourcemapStore *sourcemap.Store) common.MapStr {
	e.data = common.MapStr{}
	e.add("id", e.Id)
	e.add("page", e.Page.Fields())

	exceptionChain := flattenExceptionTree(e.Exception)
	e.addException(libraryPattern, excludeFromGrouping, sourcemapStore, exceptionChain)
	e.addLog(libraryPattern, excludeFromGrouping, sourcemapStore)
	// TODO is this needed? / why?
	if sourcemapStore != nil {
		e.updateCulprit()
	}
	e.add("culprit", e.Culprit)
	e.add("custom", e.Custom.Fields())

	e.add("grouping_key", e.calcGroupingKey(exceptionChain))

	return e.data
}

func (e *Event) updateCulprit() {
	var fr *m.StacktraceFrame
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

func findSmappedNonLibraryFrame(frames []*m.StacktraceFrame) *m.StacktraceFrame {
	for _, fr := range frames {
		if fr.IsSourcemapApplied() && !fr.IsLibraryFrame() {
			return fr
		}
	}
	return nil
}

func (e *Event) addException(libraryPattern, excludeFromGrouping *regexp.Regexp, sourcemapStore *sourcemap.Store, chain []Exception) {
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

		st := exception.Stacktrace.Transform(libraryPattern, excludeFromGrouping, sourcemapStore, e.Metadata.Service)
		utility.Set(ex, "stacktrace", st)

		result = append(result, ex)
	}

	e.add("exception", result)
}

func (e *Event) addLog(libraryPattern, excludeFromGrouping *regexp.Regexp, sourcemapStore *sourcemap.Store) {
	if e.Log == nil {
		return
	}
	log := common.MapStr{}
	utility.Set(log, "message", e.Log.Message)
	utility.Set(log, "param_message", e.Log.ParamMessage)
	utility.Set(log, "logger_name", e.Log.LoggerName)
	utility.Set(log, "level", e.Log.Level)
	st := e.Log.Stacktrace.Transform(libraryPattern, excludeFromGrouping, sourcemapStore, e.Metadata.Service)
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
func (e *Event) calcGroupingKey(chain []Exception) string {
	k := newGroupingKey()
	var stacktrace m.Stacktrace

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

func (e *Event) add(key string, val interface{}) {
	utility.Set(e.data, key, val)
}

func addStacktraceCounter(st m.Stacktrace) {
	if frames := len(st); frames > 0 {
		stacktraceCounter.Inc()
		frameCounter.Add(int64(frames))
	}
}

type exceptionDecoder func(map[string]interface{}) *Exception

func decodeException(decoder *utility.ManualDecoder) exceptionDecoder {
	var decode exceptionDecoder
	decode = func(exceptionTree map[string]interface{}) *Exception {
		exMsg := decoder.StringPtr(exceptionTree, "message")
		exType := decoder.StringPtr(exceptionTree, "type")
		if decoder.Err != nil || (exMsg == nil && exType == nil) {
			return nil
		}
		ex := Exception{
			Message:    exMsg,
			Type:       exType,
			Code:       decoder.Interface(exceptionTree, "code"),
			Module:     decoder.StringPtr(exceptionTree, "module"),
			Attributes: decoder.Interface(exceptionTree, "attributes"),
			Handled:    decoder.BoolPtr(exceptionTree, "handled"),
			Stacktrace: m.Stacktrace{},
		}
		var stacktrace *m.Stacktrace
		stacktrace, decoder.Err = m.DecodeStacktrace(exceptionTree["stacktrace"], decoder.Err)
		if stacktrace != nil {
			ex.Stacktrace = *stacktrace
		}
		for _, cause := range decoder.InterfaceArr(exceptionTree, "cause") {
			e, ok := cause.(map[string]interface{})
			if !ok {
				decoder.Err = errors.New("cause must be an exception")
				return nil
			}
			nested := decode(e)
			if nested != nil {
				ex.Cause = append(ex.Cause, *nested)
			}
		}
		return &ex
	}
	return decode
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
