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

package systemtest

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

// TestHTTP2OverTLS performs a sanity check on http2 request response flow.
func TestHTTP2OverTLS(t *testing.T) {
	srv := apmservertest.NewUnstartedServerTB(t)
	require.NoError(t, srv.StartTLS())

	tlsConf := srv.TLS.Clone()
	tlsConf.NextProtos = []string{"h2"}
	client := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   tlsConf,
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(srv.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.Equal(t, 2, resp.ProtoMajor)
	require.NotZero(t, resp.StatusCode)
}

// TestHTTP2OverTLSFramingConsistency performs a raw HTTP/2 exchange and asserts
// that SETTINGS remain consistent across the connection while a request/response
// completes without GOAWAY.
// This is to prevent reoccurrence of regression in https://github.com/elastic/apm-server/issues/20887
func TestHTTP2OverTLSFramingConsistency(t *testing.T) {
	srv := apmservertest.NewUnstartedServerTB(t)
	require.NoError(t, srv.StartTLS())

	tlsConf := srv.TLS.Clone()
	tlsConf.NextProtos = []string{"h2"}
	var (
		mu       sync.Mutex
		recorded *recordingConn
	)
	h2Transport := &http2.Transport{
		TLSClientConfig: tlsConf,
		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			conn, err := tls.DialWithDialer(&net.Dialer{}, network, addr, cfg)
			if err != nil {
				return nil, err
			}
			rc := &recordingConn{Conn: conn}
			mu.Lock()
			recorded = rc
			mu.Unlock()
			return rc, nil
		},
	}
	client := &http.Client{
		Transport: h2Transport,
		Timeout:   5 * time.Second,
	}
	resp, err := client.Get(srv.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NotZero(t, resp.StatusCode)

	mu.Lock()
	require.NotNil(t, recorded)
	serverFrames := recorded.Bytes()
	mu.Unlock()

	nonACKSettings, statusCode, sawGoAway := collectSettingsAndStatusFromBytes(t, serverFrames)

	require.False(t, sawGoAway, "server should not send GOAWAY for a valid request")
	require.NotZero(t, statusCode, "response must include a non-zero :status")
	require.GreaterOrEqual(t, len(nonACKSettings), 2, "expected sniffing and backend SETTINGS frames")

	first := nonACKSettings[0]
	for i := 1; i < len(nonACKSettings); i++ {
		require.Equalf(t, first, nonACKSettings[i], "non-ACK SETTINGS #%d differs from first SETTINGS frame", i+1)
	}
}

type recordingConn struct {
	net.Conn
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *recordingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.mu.Lock()
		_, _ = c.buf.Write(p[:n])
		c.mu.Unlock()
	}
	return n, err
}

func (c *recordingConn) Bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.buf.Bytes()...)
}

func collectSettingsAndStatusFromBytes(t *testing.T, data []byte) ([][]http2.Setting, int, bool) {
	t.Helper()

	var (
		nonACKSettings [][]http2.Setting
		sawGoAway      bool
		statusCode     int
		done           bool
		headerBlock    bytes.Buffer
	)

	decoder := hpack.NewDecoder(4096, func(hf hpack.HeaderField) {
		if hf.Name == ":status" {
			code, err := strconv.Atoi(hf.Value)
			require.NoError(t, err)
			statusCode = code
		}
	})

	decodeHeaders := func(endStream bool) {
		_, err := decoder.Write(headerBlock.Bytes())
		require.NoError(t, err)
		require.NoError(t, decoder.Close())
		headerBlock.Reset()
		if endStream {
			done = true
		}
	}
	framer := http2.NewFramer(io.Discard, bytes.NewReader(data))

	for !done && !sawGoAway {
		f, err := framer.ReadFrame()
		require.NoError(t, err)

		switch f := f.(type) {
		case *http2.SettingsFrame:
			if f.IsAck() {
				continue
			}
			var settings []http2.Setting
			require.NoError(t, f.ForeachSetting(func(s http2.Setting) error {
				settings = append(settings, s)
				return nil
			}))
			nonACKSettings = append(nonACKSettings, settings)
		case *http2.HeadersFrame:
			_, err := headerBlock.Write(f.HeaderBlockFragment())
			require.NoError(t, err)
			if f.HeadersEnded() {
				decodeHeaders(f.StreamEnded())
			}
		case *http2.ContinuationFrame:
			_, err := headerBlock.Write(f.HeaderBlockFragment())
			require.NoError(t, err)
			if f.HeadersEnded() {
				decodeHeaders(false)
			}
		case *http2.DataFrame:
			if f.StreamEnded() {
				done = true
			}
		case *http2.GoAwayFrame:
			sawGoAway = true
		}
	}

	return nonACKSettings, statusCode, sawGoAway
}
