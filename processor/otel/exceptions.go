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

// Portions copied from OpenTelemetry Collector (contrib), from the
// elastic exporter.
//
// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/apm-server/model"
)

var (
	javaStacktraceAtRegexp   = regexp.MustCompile(`at (.*)\(([^:]*)(?::([0-9]+))?\)`)
	javaStacktraceMoreRegexp = regexp.MustCompile(`\.\.\. ([0-9]+) more`)
)

func convertOpenTelemetryExceptionSpanEvent(
	timestamp time.Time,
	exceptionType, exceptionMessage, exceptionStacktrace string,
	exceptionEscaped bool,
	language string,
) *model.Error {
	if exceptionMessage == "" {
		exceptionMessage = "[EMPTY]"
	}
	exceptionHandled := !exceptionEscaped
	exceptionError := model.Error{
		Timestamp: timestamp,
		Exception: &model.Exception{
			Message: exceptionMessage,
			Type:    exceptionType,
			Handled: &exceptionHandled,
		},
	}
	if exceptionStacktrace != "" {
		if err := setExceptionStacktrace(exceptionStacktrace, language, exceptionError.Exception); err != nil {
			// Couldn't parse stacktrace, just add it as an attribute to the
			// exception so the user can still access it.
			exceptionError.Exception.Stacktrace = nil
			exceptionError.Exception.Cause = nil
			exceptionError.Exception.Attributes = map[string]interface{}{
				"stacktrace": exceptionStacktrace,
			}
		}
	}
	return &exceptionError
}

func setExceptionStacktrace(s, language string, out *model.Exception) error {
	switch language {
	case "java":
		return setJavaExceptionStacktrace(s, out)
	}
	return fmt.Errorf("parsing %q stacktraces not implemented", language)
}

func setJavaExceptionStacktrace(s string, out *model.Exception) error {
	const (
		causedByPrefix   = "Caused by: "
		suppressedPrefix = "Suppressed: "
	)

	type Exception struct {
		*model.Exception
		enclosing *model.Exception
		indent    int
	}
	first := true
	current := Exception{out, nil, 0}
	stack := []Exception{}
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		if first {
			// Ignore the first line, we only care about the locations.
			first = false
			continue
		}
		var indent int
		line := scanner.Text()
		if i := strings.IndexFunc(line, isNotTab); i > 0 {
			line = line[i:]
			indent = i
		}
		for indent < current.indent {
			n := len(stack)
			current, stack = stack[n-1], stack[:n-1]
		}
		switch {
		case strings.HasPrefix(line, "at "):
			if err := parseJavaStacktraceFrame(line, current.Exception); err != nil {
				return err
			}
		case strings.HasPrefix(line, "..."):
			// "... N more" lines indicate that the last N frames from the enclosing
			// exception's stacktrace are common to this exception.
			if current.enclosing == nil {
				return fmt.Errorf("no enclosing exception preceding line %q", line)
			}
			submatch := javaStacktraceMoreRegexp.FindStringSubmatch(line)
			if submatch == nil {
				return fmt.Errorf("failed to parse stacktrace line %q", line)
			}
			if n, err := strconv.Atoi(submatch[1]); err == nil {
				enclosing := current.enclosing
				if len(enclosing.Stacktrace) < n {
					return fmt.Errorf(
						"enclosing exception stacktrace has %d frames, cannot satisfy %q",
						len(enclosing.Stacktrace), line,
					)
				}
				m := len(enclosing.Stacktrace)
				current.Stacktrace = append(current.Stacktrace, enclosing.Stacktrace[m-n:]...)
			}
		case strings.HasPrefix(line, causedByPrefix):
			// "Caused by:" lines are at the same level of indentation
			// as the enclosing exception.
			current.Cause = make([]model.Exception, 1)
			current.enclosing = current.Exception
			current.Exception = &current.Cause[0]
			current.Exception.Handled = current.enclosing.Handled
			current.Message = line[len(causedByPrefix):]
		case strings.HasPrefix(line, suppressedPrefix):
			// Suppressed exceptions have no place in the Elastic APM
			// model, so they are ignored.
			//
			// Unlike "Caused by:", "Suppressed:" lines are indented within their
			// enclosing exception; we just account for the indentation here.
			stack = append(stack, current)
			current.enclosing = current.Exception
			current.Exception = &model.Exception{}
			current.indent = indent
		default:
			return fmt.Errorf("unexpected line %q", line)
		}
	}
	return scanner.Err()
}

func parseJavaStacktraceFrame(s string, out *model.Exception) error {
	submatch := javaStacktraceAtRegexp.FindStringSubmatch(s)
	if submatch == nil {
		return fmt.Errorf("failed to parse stacktrace line %q", s)
	}
	var module string
	function := submatch[1]
	if slash := strings.IndexRune(function, '/'); slash >= 0 {
		// We could have either:
		//  - "class_loader/module/class.method"
		//  - "module/class.method"
		module, function = function[:slash], function[slash+1:]
		if slash := strings.IndexRune(function, '/'); slash >= 0 {
			module, function = function[:slash], function[slash+1:]
		}
	}
	var classname string
	if dot := strings.LastIndexByte(function, '.'); dot > 0 {
		// Split into classname and method.
		classname, function = function[:dot], function[dot+1:]
	}
	file := submatch[2]
	var lineno *int
	if submatch[3] != "" {
		if n, err := strconv.Atoi(submatch[3]); err == nil {
			lineno = &n
		}
	}
	out.Stacktrace = append(out.Stacktrace, &model.StacktraceFrame{
		Module:    module,
		Classname: classname,
		Function:  function,
		Filename:  file,
		Lineno:    lineno,
	})
	return nil
}

func isNotTab(r rune) bool {
	return r != '\t'
}
