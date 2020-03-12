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

package beater

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

/*
Run the benchmarks as follows:

	$ go test beater/*.go -run=XXX -bench=. -cpuprofile=cpu.out

then load the cpu profile file:

	$ go tool pprof beater.test cpu.out

type `web` to get a nice svg that shows the call graph and time spent:

	(pprof) web

To get a memory profile, use this:

	$ go test beater/*.go -run=XXX -bench=. -memprofile=mem.out

*/

func (bt *beater) client(insecure bool) (string, *http.Client) {
	transport := &http.Transport{}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	bt.mutex.Lock() // for reading bt.server
	defer bt.mutex.Unlock()
	if parsed, err := url.Parse(bt.server.httpServer.Addr); err == nil && parsed.Scheme == "unix" {
		transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", parsed.Path)
		}
		return "http://test-apm-server/", &http.Client{
			Transport: transport,
		}
	}
	scheme := "http://"
	if bt.config.TLS.IsEnabled() {
		scheme = "https://"
	}
	return scheme + bt.config.Host, &http.Client{Transport: transport}
}

func (bt *beater) wait() error {
	wait := make(chan struct{}, 1)

	go func() {
		for {
			bt.mutex.Lock()
			if bt.server.httpServer != nil {
				bt.mutex.Unlock()
				break
			}
			bt.mutex.Unlock()
			time.Sleep(10 * time.Millisecond)
		}
		wait <- struct{}{}
	}()
	timeout := time.NewTimer(2 * time.Second)

	select {
	case <-wait:
		return nil
	case <-timeout.C:
		return errors.New("timeout waiting server create")
	}
}

func setupBeater(t *testing.T, apmBeat *beat.Beat, ucfg *common.Config, beatConfig *beat.BeatConfig) (*beater, func(), error) {
	// create our beater
	beatBeater, err := New(apmBeat, ucfg)
	if err != nil {
		return nil, nil, err
	}
	require.NotNil(t, beatBeater)

	c := make(chan error)
	// start it
	go func() {
		err := beatBeater.Run(apmBeat)
		if err != nil {
			c <- err
		}
	}()

	btr := beatBeater.(*beater)
	if err := btr.wait(); err != nil {
		return nil, nil, err
	}

	url, client := btr.client(true)
	go func() {
		waitForServer(url, client, c)
	}()
	select {
	case err := <-c:
		return btr, beatBeater.Stop, err
	case <-time.After(time.Second * 10):
		return nil, nil, errors.New("timeout waiting for server start")
	}
}
