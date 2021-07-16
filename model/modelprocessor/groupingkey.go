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

package modelprocessor

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"hash"
	"io"

	"github.com/elastic/apm-server/model"
)

// SetGroupingKey is a model.BatchProcessor that sets the grouping key for errors
// by hashing their stack frames.
type SetGroupingKey struct{}

// ProcessBatch sets the grouping key for errors.
func (s SetGroupingKey) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		if event.Error != nil {
			s.processError(ctx, event.Error)
		}
	}
	return nil
}

func (s SetGroupingKey) processError(ctx context.Context, event *model.Error) {
	hash := md5.New()
	var updated bool
	if event.Exception != nil {
		if s.hashExceptionTree(event.Exception, hash, s.hashExceptionType) {
			updated = true
		}
	}
	if event.Log != nil {
		if s.maybeWriteString(event.Log.ParamMessage, hash) {
			updated = true
		}
	}
	var haveExceptionStacktrace bool
	if event.Exception != nil {
		haveExceptionStacktrace = s.hashExceptionTree(event.Exception, hash, s.hashExceptionStacktrace)
		updated = updated || haveExceptionStacktrace
	}
	if !haveExceptionStacktrace && event.Log != nil {
		if s.hashStacktrace(event.Log.Stacktrace, hash) {
			updated = true
		}
	}
	if !updated {
		if event.Exception != nil {
			updated = s.hashExceptionTree(event.Exception, hash, s.hashExceptionMessage)
		}
		if !updated && event.Log != nil {
			s.maybeWriteString(event.Log.Message, hash)
		}
	}
	event.GroupingKey = hex.EncodeToString(hash.Sum(nil))
}

func (s SetGroupingKey) hashExceptionTree(e *model.Exception, out hash.Hash, f func(*model.Exception, hash.Hash) bool) bool {
	updated := f(e, out)
	for _, cause := range e.Cause {
		if s.hashExceptionTree(&cause, out, f) {
			updated = true
		}
	}
	return updated
}

func (s SetGroupingKey) hashExceptionType(e *model.Exception, out hash.Hash) bool {
	return s.maybeWriteString(e.Type, out)
}

func (s SetGroupingKey) hashExceptionMessage(e *model.Exception, out hash.Hash) bool {
	return s.maybeWriteString(e.Message, out)
}

func (s SetGroupingKey) hashExceptionStacktrace(e *model.Exception, out hash.Hash) bool {
	return s.hashStacktrace(e.Stacktrace, out)
}

func (s SetGroupingKey) hashStacktrace(stacktrace model.Stacktrace, out hash.Hash) bool {
	var updated bool
	for _, frame := range stacktrace {
		if frame.ExcludeFromGrouping {
			continue
		}
		switch {
		case frame.Module != "":
			io.WriteString(out, frame.Module)
			updated = true
		case frame.Filename != "":
			io.WriteString(out, frame.Filename)
			updated = true
		case frame.Classname != "":
			io.WriteString(out, frame.Classname)
			updated = true
		}
		if s.maybeWriteString(frame.Function, out) {
			updated = true
		}
	}
	return updated
}

func (SetGroupingKey) maybeWriteString(s string, out hash.Hash) bool {
	if s == "" {
		return false
	}
	io.WriteString(out, s)
	return true
}
