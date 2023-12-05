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

package systemtest_test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest/apmservertest"
	lumberserver "github.com/elastic/go-lumber/server"
)

func TestLogstashOutput(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() }) // ignore error; ls.Close closes it too

	ls, err := lumberserver.NewWithListener(listener, lumberserver.V2(true))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, ls.Close()) })

	waitForIntegration := false
	srv := apmservertest.NewUnstartedServerTB(t,
		"-E", "queue.mem.events=20000",
		"-E", "queue.mem.flush.min_events=10000",
		"-E", "queue.mem.flush.timeout=60s",
	)
	srv.Config.WaitForIntegration = &waitForIntegration
	srv.Config.Output = apmservertest.OutputConfig{
		Logstash: &apmservertest.LogstashOutputConfig{
			Enabled:     true,
			BulkMaxSize: 5000,
			Hosts:       []string{listener.Addr().String()},
		},
	}
	require.NoError(t, srv.Start())

	// Send 20000 events. We should receive 4 batches of 5000.
	tracer := srv.Tracer()
	for i := 0; i < 20; i++ {
		for i := 0; i < 1000; i++ {
			tx := tracer.StartTransaction("name", "type")
			tx.End()
		}
		tracer.Flush(nil)
	}
	for i := 0; i < 4; i++ {
		select {
		case batch := <-ls.ReceiveChan():
			batch.ACK()
			assert.Len(t, batch.Events, 5000)
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for batch")
		}
	}
	// Should be no more batches.
	select {
	case <-ls.ReceiveChan():
		t.Error("unexpected batch received")
	case <-time.After(100 * time.Millisecond):
	}
}
