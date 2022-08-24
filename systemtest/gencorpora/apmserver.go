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

package gencorpora

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

// APMServer represents a wrapped exec CMD for APMServer process
type APMServer struct {
	apmHost string
	cmd     *apmservertest.ServerCmd
	mu      sync.Mutex
}

// NewAPMServer returns an apmservertest.Server that sends data to esHost
// using the Elasticsearch output.
func NewAPMServer(ctx context.Context, esHost string) *apmservertest.Server {
	srv := apmservertest.NewUnstartedServer()
	waitForIntegration := false
	srv.Config.WaitForIntegration = &waitForIntegration
	srv.Config.Output.Elasticsearch.Hosts = []string{esHost}
	srv.Config.Kibana = nil
	return srv
}

// StreamAPMServerLogs streams logs from the apmservertest.Server process to stderr.
//
// The method must be called after the server is started, and only one active
// StreamAPMServerLogs call is allowed per server.
func StreamAPMServerLogs(ctx context.Context, srv *apmservertest.Server) error {
	logger, err := zap.NewDevelopment(zap.IncreaseLevel(gencorporaConfig.LoggingLevel))
	if err != nil {
		return err
	}
	iter := srv.Logs.Iterator()
	defer iter.Close()
	var entry apmservertest.LogEntry
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case entry = <-iter.C():
		}
		if ce := logger.Check(entry.Level, entry.Message); ce != nil {
			fields := make([]zapcore.Field, 0, len(entry.Fields))
			for k, v := range entry.Fields {
				fields = append(fields, zap.Any(k, v))
			}
			ce.Time = entry.Timestamp
			ce.LoggerName = entry.Logger
			ce.Caller = zapcore.NewEntryCaller(0, entry.File, entry.Line, entry.File != "")
			ce.Write(fields...)
		}
	}
}
