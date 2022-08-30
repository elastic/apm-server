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

package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"go.elastic.co/apm/v2"
	"go.elastic.co/apm/v2/apmtest"

	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/logp/configure"
	"github.com/elastic/elastic-agent-libs/mapstr"

	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/logs"
)

func TestLogMiddleware(t *testing.T) {

	testCases := []struct {
		name, message string
		level         zapcore.Level
		handler       request.Handler
		code          int
		traced        bool
		ecsKeys       []string
	}{
		{
			name:    "Accepted",
			message: "request accepted",
			level:   zapcore.InfoLevel,
			handler: Handler202,
			code:    http.StatusAccepted,
			ecsKeys: []string{"url.original"},
		},
		{
			name:    "Traced",
			message: "request accepted",
			level:   zapcore.InfoLevel,
			handler: Handler202,
			code:    http.StatusAccepted,
			ecsKeys: []string{"url.original", "trace.id", "transaction.id"},
			traced:  true,
		},
		{
			name:    "Error",
			message: "forbidden request",
			level:   zapcore.ErrorLevel,
			handler: Handler403,
			code:    http.StatusForbidden,
			ecsKeys: []string{"url.original", "error.message"},
		},
		{
			name:    "Panic",
			message: "internal error",
			level:   zapcore.ErrorLevel,
			handler: Apply(RecoverPanicMiddleware(), HandlerPanic),
			code:    http.StatusInternalServerError,
			ecsKeys: []string{"url.original", "error.message", "error.stack_trace"},
		},
		{
			name:    "Error without keyword",
			message: "handled request",
			level:   zapcore.ErrorLevel,
			handler: func(c *request.Context) {
				c.Result.StatusCode = http.StatusForbidden
				c.WriteResult()
			},
			code:    http.StatusForbidden,
			ecsKeys: []string{"url.original"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// log setup
			configure.Logging("APM Server test",
				agentconfig.MustNewConfigFrom(`{"ecs":true}`))
			require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))

			// prepare and record request
			c, rec := DefaultContextWithResponseRecorder()
			c.Request.Header.Set(headers.UserAgent, tc.name)
			if tc.traced {
				tx := apmtest.DiscardTracer.StartTransaction("name", "type")
				c.Request = c.Request.WithContext(apm.ContextWithTransaction(c.Request.Context(), tx))
				defer tx.End()
			}
			Apply(LogMiddleware(), tc.handler)(c)

			// check log lines
			assert.Equal(t, tc.code, rec.Code)
			entries := logp.ObserverLogs().TakeAll()
			require.Equal(t, 1, len(entries))
			entry := entries[0]
			assert.Equal(t, logs.Request, entry.LoggerName)
			assert.Equal(t, tc.level, entry.Level)
			assert.Equal(t, tc.message, entry.Message)

			encoder := zapcore.NewMapObjectEncoder()
			ec := mapstr.M{}
			for _, f := range entry.Context {
				f.AddTo(encoder)
				ec.DeepUpdate(encoder.Fields)
			}
			keys := []string{"http.request.id", "http.request.method", "http.request.body.bytes",
				"source.address", "user_agent.original", "http.response.status_code", "event.duration"}
			keys = append(keys, tc.ecsKeys...)
			for _, key := range keys {
				ok, _ := ec.HasKey(key)
				assert.True(t, ok, key)
			}
		})
	}
}
