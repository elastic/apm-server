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

package beatcmd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/elastic/elastic-agent-libs/logp"
)

func TestAdjustMaxProcsTickerRefresh(t *testing.T) {
	// This test asserts that the GOMAXPROCS is called multiple times
	// respecting the time.Duration that is passed in the function.
	for _, maxP := range []int{2, 4, 8} {
		t.Run(fmt.Sprintf("%d_GOMAXPROCS", maxP), func(t *testing.T) {
			observedLogs := testAdjustMaxProcs(t, maxP, false)
			assert.GreaterOrEqual(t, observedLogs.Len(), 10)
		})
	}
}

func TestAdjustMaxProcsTickerRefreshDiffLogger(t *testing.T) {
	// This test asserts that the log messages aren't logged more than once.
	for _, maxP := range []int{2, 4, 8} {
		t.Run(fmt.Sprintf("%d_GOMAXPROCS", maxP), func(t *testing.T) {
			observedLogs := testAdjustMaxProcs(t, maxP, true)
			// Assert that only 1 message has been logged.
			assert.Equal(t, observedLogs.Len(), 1)
		})
	}
}

func testAdjustMaxProcs(t *testing.T, maxP int, diffCore bool) *observer.ObservedLogs {
	t.Setenv("GOMAXPROCS", fmt.Sprint(maxP))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	// Adjust maxprocs every 1ms.
	refreshDuration := time.Millisecond
	logFunc := logger.Infof
	if diffCore {
		logFunc = diffInfof(logger)
	}

	go adjustMaxProcs(ctx, refreshDuration, logFunc, logger.Errorf)

	filterMsg := fmt.Sprintf(`maxprocs: Honoring GOMAXPROCS="%d"`, maxP)
	for {
		select {
		// Wait for 50ms so adjustmaxprocs has had time to run a few times.
		case <-time.After(50 * refreshDuration):
			logs := observedLogs.FilterMessageSnippet(filterMsg)
			if logs.Len() >= 1 {
				return logs
			}
		case <-ctx.Done():
			t.Error(ctx.Err())
			return nil
		}
	}
}
