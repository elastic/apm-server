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
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-perf/loadgen"
)

// Run runs APM-Server and CatBulk server followed by sending
// load to APM-Server and writing the generated corpus to a file.
// Run exits on error or on successful corpora generation.
func Run(rootCtx context.Context) error {
	// Create fake ES server
	esServer, err := NewCatBulkServer()
	if err != nil {
		return err
	}

	// Create APM-Server to send documents to fake ES
	apmServer := NewAPMServer(rootCtx, esServer.Addr)
	if err := apmServer.Start(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(rootCtx)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		<-gctx.Done()

		var result error
		if err := apmServer.Close(); err != nil {
			result = multierror.Append(result, err)
		}
		if err := esServer.Stop(); err != nil {
			result = multierror.Append(result, err)
		}

		return result
	})
	g.Go(esServer.Serve)
	g.Go(func() error {
		return StreamAPMServerLogs(ctx, apmServer)
	})
	g.Go(func() error {
		defer cancel()

		waitCtx, waitCancel := context.WithTimeout(gctx, 10*time.Second)
		defer waitCancel()
		if err := apmServer.WaitForPublishReady(waitCtx); err != nil {
			return fmt.Errorf("failed while waiting for APM-Server to be ready: %w", err)
		}

		return generateLoad(ctx, apmServer.URL, gencorporaConfig.ReplayCount)
	})

	if err := g.Wait(); !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func generateLoad(ctx context.Context, serverURL string, replayCount int) error {
	inf := loadgen.GetNewLimiter(0, 0)
	handler, err := loadgen.NewEventHandler(loadgen.EventHandlerParams{
		Path:    `*.ndjson`,
		URL:     serverURL,
		Limiter: inf,
	})
	if err != nil {
		return err
	}

	for i := 0; i < replayCount; i++ {
		if _, err = handler.SendBatches(ctx); err != nil {
			return fmt.Errorf("failed sending batches on iteration %d: %w", i+1, err)
		}
	}
	return nil
}
