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
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestTLSConfig(t *testing.T) {
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.TLS = &apmservertest.TLSConfig{
		SupportedProtocols: []string{"TLSv1.2"},
		CipherSuites:       []string{"ECDHE-RSA-AES-128-GCM-SHA256"},
	}
	require.NoError(t, srv.StartTLS())

	attemptRequest := func(t *testing.T, minVersion, maxVersion uint16, cipherSuites ...uint16) error {
		tlsConfig := &tls.Config{RootCAs: srv.TLS.RootCAs}
		tlsConfig.MinVersion = minVersion
		tlsConfig.MaxVersion = maxVersion
		tlsConfig.CipherSuites = cipherSuites
		httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
		resp, err := httpClient.Get(srv.URL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	}

	t.Run("compatible_cipher_suite", func(t *testing.T) {
		err := attemptRequest(t, 0, 0, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256)
		require.NoError(t, err)
	})

	t.Run("compatible_protocol", func(t *testing.T) {
		err := attemptRequest(t, tls.VersionTLS12, tls.VersionTLS12)
		require.NoError(t, err)
	})

	t.Run("incompatible_cipher_suite", func(t *testing.T) {
		err := attemptRequest(t, 0, 0, tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384)
		require.Error(t, err)
		assert.Regexp(t, ".*tls: handshake failure", err.Error())
	})

	t.Run("incompatible_protocol", func(t *testing.T) {
		for _, version := range []uint16{tls.VersionTLS10, tls.VersionTLS13} {
			err := attemptRequest(t, version, version)
			require.Error(t, err)
			assert.Regexp(t, ".*tls: protocol version not supported", err.Error())
		}
	})
}

func TestTLSClientAuth(t *testing.T) {
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.TLS = &apmservertest.TLSConfig{ClientAuthentication: "required"}
	require.NoError(t, srv.StartTLS())

	attemptRequest := func(t *testing.T, tlsConfig *tls.Config) error {
		httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
		resp, err := httpClient.Get(srv.URL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	}

	err := attemptRequest(t, &tls.Config{InsecureSkipVerify: true})
	require.Error(t, err)
	assert.Regexp(t, "tls: bad certificate", err.Error())
	logs := srv.Logs.Iterator()
	defer logs.Close()
	for entry := range logs.C() {
		if entry.Logger != "beater.http" || entry.Level != zapcore.ErrorLevel {
			continue
		}
		assert.Equal(t, "http/server.go", entry.File)
		assert.Regexp(t, "tls: client didn't provide a certificate", entry.Message)
		break
	}

	err = attemptRequest(t, srv.TLS)
	require.NoError(t, err)
}
