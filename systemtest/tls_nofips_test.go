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

//go:build !requirefips

package systemtest_test

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestDefaultIncompatibleTLSConfig(t *testing.T) {
	srv := apmservertest.NewUnstartedServerTB(t)
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

	t.Run("incompatible_cipher_suite", func(t *testing.T) {
		err := attemptRequest(t, tls.VersionTLS12, tls.VersionTLS12, tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA)
		require.Error(t, err)
		assert.Regexp(t, ".*tls: handshake failure", err.Error())
	})

	t.Run("incompatible_protocol", func(t *testing.T) {
		err := attemptRequest(t, tls.VersionTLS10, tls.VersionTLS10)
		require.Error(t, err)
		assert.Regexp(t, ".*tls: protocol version not supported", err.Error())
	})
}
