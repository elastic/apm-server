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
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

type testBeater struct {
	*beater

	listenAddr string
	baseURL    string
	client     *http.Client
}

func setupBeater(t *testing.T, apmBeat *beat.Beat, ucfg *common.Config, beatConfig *beat.BeatConfig) (*testBeater, error) {
	onboardingDocs := make(chan onboardingDoc, 1)
	createBeater := NewCreator(CreatorParams{
		RunServer: func(ctx context.Context, args ServerParams) error {
			// Wrap the reporter so we can intercept the
			// onboarding doc, to extract the listen address.
			origReporter := args.Reporter
			args.Reporter = func(ctx context.Context, req publish.PendingReq) error {
				for _, tf := range req.Transformables {
					if o, ok := tf.(onboardingDoc); ok {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case onboardingDocs <- o:
						}
					}
				}
				return origReporter(ctx, req)
			}
			return RunServer(ctx, args)
		},
	})

	// create our beater
	beatBeater, err := createBeater(apmBeat, ucfg)
	if err != nil {
		return nil, err
	}
	require.NotNil(t, beatBeater)

	errCh := make(chan error)
	go func() {
		err := beatBeater.Run(apmBeat)
		if err != nil {
			errCh <- err
		}
	}()

	tb := &testBeater{beater: beatBeater.(*beater)}
	select {
	case err := <-errCh:
		return nil, err
	case o := <-onboardingDocs:
		tb.initClient(tb.config, o.listenAddr)
	case <-time.After(time.Second * 10):
		return nil, errors.New("timeout waiting for server to start listening")
	}

	res, err := tb.client.Get(tb.baseURL)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	return tb, nil
}

func (tb *testBeater) initClient(cfg *config.Config, listenAddr string) {
	tb.listenAddr = listenAddr
	transport := &http.Transport{}
	if parsed, err := url.Parse(cfg.Host); err == nil && parsed.Scheme == "unix" {
		transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", parsed.Path)
		}
		tb.baseURL = "http://test-apm-server/"
		tb.client = &http.Client{Transport: transport}
	} else {
		scheme := "http://"
		if cfg.TLS.IsEnabled() {
			scheme = "https://"
		}
		tb.baseURL = scheme + listenAddr
		tb.client = &http.Client{Transport: transport}
	}
}
