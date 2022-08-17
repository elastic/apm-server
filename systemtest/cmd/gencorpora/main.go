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

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/systemtest/gencorpora"
	"github.com/elastic/apm-server/systemtest/loadgen"
)

const apmHost = "127.0.0.1:8200"

func run(rootCtx context.Context) error {
	// Create fake ES server
	esServer, err := gencorpora.GetCatBulkServer()
	if err != nil {
		return err
	}

	// Create APM-Server to send documents to fake ES
	apmServer := gencorpora.GetAPMServer(rootCtx, esServer.Addr, apmHost)
	apmServer.StreamLogs(rootCtx)

	ctx, cancel := context.WithCancel(rootCtx)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		<-gctx.Done()

		var result error
		if err := apmServer.Stop(); err != nil {
			result = multierror.Append(result, err)
		}
		if err := esServer.Stop(); err != nil {
			result = multierror.Append(result, err)
		}

		return result
	})
	g.Go(esServer.Start)
	g.Go(apmServer.Start)
	g.Go(func() error {
		defer cancel()

		waitCtx, waitCancel := context.WithTimeout(gctx, 10*time.Second)
		defer waitCancel()
		if err := apmServer.WaitForPublishReady(waitCtx); err != nil {
			return fmt.Errorf("failed while waiting for APM-Server to be ready with err %v", err)
		}

		return generateLoad(ctx, "http://127.0.0.1:8200")
	})

	if err := g.Wait(); !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func generateLoad(ctx context.Context, serverURL string) error {
	inf := loadgen.GetNewLimiter(0)
	handler, err := loadgen.NewEventHandler(`*.ndjson`, serverURL, "", inf)
	if err != nil {
		return err
	}

	_, err = handler.SendBatches(ctx)
	return err
}

func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}
