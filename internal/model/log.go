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

import "github.com/elastic/elastic-agent-libs/mapstr"

const (
	AppLogsDataset = "apm.app"
)

var (
	// LogProcessor is the Processor value that should be assigned to log events.
	LogProcessor = Processor{Name: "log", Event: "log"}
)

// Log holds information about a log, as defined by ECS.
//
// https://www.elastic.co/guide/en/ecs/current/ecs-log.html
type Log struct {
	// Level holds the log level of the log event.
	Level string
	// Logger holds the name of the logger instance.
	Logger string
	Origin LogOrigin
}

// LogOrigin holds information about the origin of the log.
type LogOrigin struct {
	LogFile      LogOriginFile
	FunctionName string
}

// LogOriginFile holds information about the file and the line of the origin of the log.
type LogOriginFile struct {
	Name string
	Line int
}

func (e Log) fields() mapstr.M {
	var fields, origin, file mapStr
	fields.maybeSetString("level", e.Level)
	fields.maybeSetString("logger", e.Logger)
	origin.maybeSetString("function", e.Origin.FunctionName)
	file.maybeSetString("name", e.Origin.LogFile.Name)
	if e.Origin.LogFile.Line > 0 {
		file.set("line", e.Origin.LogFile.Line)
	}
	origin.maybeSetMapStr("file", mapstr.M(file))
	fields.maybeSetMapStr("origin", mapstr.M(origin))
	return mapstr.M(fields)
}
