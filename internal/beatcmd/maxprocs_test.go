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
	"golang.org/x/sync/errgroup"

	"github.com/elastic/elastic-agent-libs/logp"
)

func TestAdjustMaxProcs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	defer g.Wait()
	defer cancel()

	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	expectAdjustment := func(n int) {
		// Wait for GOMAXPROCS to be updated, and ensure only a single log message is logged.
		filterMsg := fmt.Sprintf(`maxprocs: Honoring GOMAXPROCS="%d"`, n)
		deadline := time.Now().Add(10 * time.Second)
		for {
			if time.Now().After(deadline) {
				t.Error("timed out waiting for GOMAXPROCS to be set")
				return
			}
			logs := observedLogs.FilterMessageSnippet(filterMsg)
			if logs.Len() >= 1 {
				assert.Len(t, observedLogs.TakeAll(), 1)
				break
			}
		}

		// Duplicate logs should be suppressed.
		time.Sleep(50 * time.Millisecond)
		logs := observedLogs.FilterMessageSnippet(filterMsg)
		assert.Zero(t, logs.Len(), logs)
	}

	// Adjust maxprocs every 1ms. We set GOMAXPROCS up front
	// to handle the initial adjustment which runs before the
	// loop kicks in.
	t.Setenv("GOMAXPROCS", "3") // Set before calling
	refreshDuration := time.Millisecond
	g.Go(func() error {
		return adjustMaxProcs(ctx, refreshDuration, logger)
	})
	expectAdjustment(3)
	t.Setenv("GOMAXPROCS", "7")
	expectAdjustment(7)
}
