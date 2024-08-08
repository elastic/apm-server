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

package otlp

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/elastic/apm-data/model/modelpb"
)

var (
	javaStacktraceAtRegexp   = regexp.MustCompile(`at (.*)\(([^:]*)(?::([0-9]+))?\)`)
	javaStacktraceMoreRegexp = regexp.MustCompile(`\.\.\. ([0-9]+) more`)
)

const (
	emptyExceptionMsg = "[EMPTY]"
)

// convertOpenTelemetryExceptionSpanEvent creates an otel Exception event
// from the specified arguments.
//
// OpenTelemetry semantic convention require the presence of at least one
// of the following attributes:
// - exception.type
// - exception.message
// https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-spans/#attributes
//
// To fulfill this requirement we do not set exception.type if empty but
// we always set exception.message defaulting to a constant value if empty.
func convertOpenTelemetryExceptionSpanEvent(
	exceptionType, exceptionMessage, exceptionStacktrace string,
	exceptionEscaped bool,
	language string,
) *modelpb.Error {
	if exceptionMessage == "" {
		exceptionMessage = emptyExceptionMsg
	}
	exceptionHandled := !exceptionEscaped
	exceptionError := modelpb.Error{}
	exceptionError.Exception = &modelpb.Exception{}
	exceptionError.Exception.Message = exceptionMessage
	if exceptionType != "" {
		exceptionError.Exception.Type = exceptionType
	}
	exceptionError.Exception.Handled = &exceptionHandled
	if id, err := newUniqueID(); err == nil {
		exceptionError.Id = id
	}
	if exceptionStacktrace != "" {
		if err := setExceptionStacktrace(exceptionStacktrace, language, exceptionError.Exception); err != nil {
			// Couldn't parse stacktrace, set it as `error.stack_trace` instead.
			exceptionError.Exception.Stacktrace = nil
			exceptionError.Exception.Cause = nil
			exceptionError.StackTrace = exceptionStacktrace
		}
	}
	return &exceptionError
}

func setExceptionStacktrace(s, language string, out *modelpb.Exception) error {
	switch language {
	case "java":
		return setJavaExceptionStacktrace(s, out)
	}
	return fmt.Errorf("parsing %q stacktraces not implemented", language)
}

// setJavaExceptionStacktrace parses a Java exception stack trace according to
// https://docs.oracle.com/javase/7/docs/api/java/lang/Throwable.html#printStackTrace()
func setJavaExceptionStacktrace(s string, out *modelpb.Exception) error {
	const (
		causedByPrefix   = "Caused by: "
		suppressedPrefix = "Suppressed: "
	)

	type Exception struct {
		*modelpb.Exception
		enclosing *modelpb.Exception
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
			current.Cause = make([]*modelpb.Exception, 1)
			current.Cause[0] = &modelpb.Exception{}
			current.enclosing = current.Exception
			current.Exception = current.Cause[0]
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
			current.Exception = &modelpb.Exception{}
			current.indent = indent
		default:
			return fmt.Errorf("unexpected line %q", line)
		}
	}
	return scanner.Err()
}

func parseJavaStacktraceFrame(s string, out *modelpb.Exception) error {
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
	var lineno *uint32
	if submatch[3] != "" {
		if n, err := strconv.ParseUint(submatch[3], 10, 32); err == nil {
			un := uint32(n)
			lineno = &un
		}
	}
	sf := modelpb.StacktraceFrame{}
	sf.Module = module
	sf.Classname = classname
	sf.Function = function
	sf.Filename = file
	sf.Lineno = lineno
	out.Stacktrace = append(out.Stacktrace, &sf)
	return nil
}

func isNotTab(r rune) bool {
	return r != '\t'
}

func newUniqueID() (string, error) {
	var u [16]byte
	if _, err := io.ReadFull(rand.Reader, u[:]); err != nil {
		return "", err
	}

	// convert to string
	buf := make([]byte, 32)
	hex.Encode(buf, u[:])

	return string(buf), nil
}
