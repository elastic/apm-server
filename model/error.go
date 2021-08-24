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
	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	// ErrorProcessor is the Processor value that should be assigned to error events.
	ErrorProcessor = Processor{Name: "error", Event: "error"}
)

const (
	ErrorsDataset = "apm.error"
)

type Error struct {
	ID            string
	TransactionID string
	ParentID      string

	GroupingKey string
	Culprit     string
	HTTP        *HTTP
	Custom      common.MapStr

	Exception *Exception
	Log       *Log

	TransactionSampled *bool
	TransactionType    string

	Experimental interface{}
}

type Exception struct {
	Message    string
	Module     string
	Code       string
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

func (e *Error) fields() common.MapStr {
	var fields mapStr
	if e.HTTP != nil {
		fields.maybeSetMapStr("http", e.HTTP.transactionTopLevelFields())
	}
	if e.Experimental != nil {
		fields.set("experimental", e.Experimental)
	}

	// sampled and type is nil if an error happens outside a transaction or an (old) agent is not sending sampled info
	// agents must send semantically correct data
	var transaction mapStr
	transaction.maybeSetString("id", e.TransactionID)
	transaction.maybeSetString("type", e.TransactionType)
	transaction.maybeSetBool("sampled", e.TransactionSampled)
	fields.maybeSetMapStr("transaction", common.MapStr(transaction))

	var parent mapStr
	parent.maybeSetString("id", e.ParentID)
	fields.maybeSetMapStr("parent", common.MapStr(parent))

	var errorFields mapStr
	errorFields.maybeSetString("id", e.ID)
	exceptionChain := flattenExceptionTree(e.Exception)
	if exception := e.exceptionFields(exceptionChain); len(exception) > 0 {
		errorFields.set("exception", exception)
	}
	errorFields.maybeSetMapStr("log", e.logFields())
	errorFields.maybeSetString("culprit", e.Culprit)
	errorFields.maybeSetMapStr("custom", customFields(e.Custom))
	errorFields.maybeSetString("grouping_key", e.GroupingKey)
	fields.set("error", common.MapStr(errorFields))
	return common.MapStr(fields)
}

func (e *Error) exceptionFields(chain []Exception) []common.MapStr {
	result := make([]common.MapStr, len(chain))
	for i, exception := range chain {
		var ex mapStr
		ex.maybeSetString("message", exception.Message)
		ex.maybeSetString("module", exception.Module)
		ex.maybeSetString("type", exception.Type)
		ex.maybeSetString("code", exception.Code)
		ex.maybeSetBool("handled", exception.Handled)
		if exception.Parent != nil {
			ex.set("parent", exception.Parent)
		}
		if exception.Attributes != nil {
			ex.set("attributes", exception.Attributes)
		}
		if n := len(exception.Stacktrace); n > 0 {
			frames := make([]common.MapStr, n)
			for i, frame := range exception.Stacktrace {
				frames[i] = frame.transform()
			}
			ex.set("stacktrace", frames)
		}
		result[i] = common.MapStr(ex)
	}
	return result
}

func (e *Error) logFields() common.MapStr {
	if e.Log == nil {
		return nil
	}
	var log mapStr
	log.maybeSetString("message", e.Log.Message)
	log.maybeSetString("param_message", e.Log.ParamMessage)
	log.maybeSetString("logger_name", e.Log.LoggerName)
	log.maybeSetString("level", e.Log.Level)
	if st := e.Log.Stacktrace.transform(); len(st) > 0 {
		log.set("stacktrace", st)
	}
	return common.MapStr(log)
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
