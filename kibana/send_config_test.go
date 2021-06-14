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

package kibana

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
)

func TestFlatten(t *testing.T) {
	dc := config.DefaultConfig()
	enabled := true
	dc.TLS = &tlscommon.ServerConfig{
		Enabled:          &enabled,
		VerificationMode: tlscommon.VerifyStrict,
		Certificate: tlscommon.CertificateConfig{
			Certificate: "my-cert",
			Key:         "my-key",
		},
	}
	tlsFieldsCount := 0
	cc := common.NewConfig()
	err := cc.Merge(dc)
	require.NoError(t, err)

	flat, err := flattenAndClean(cc)
	assert.NoError(t, err)

	for k := range flat {
		assert.NotContains(t, k, "elasticsearch")
		assert.NotContains(t, k, "kibana")
		assert.NotContains(t, k, "instrumentation")
		if strings.HasPrefix(k, "ssl.") {
			switch k[4:] {
			case "enabled", "certificate", "key":
				tlsFieldsCount += 1
			default:
				assert.Fail(t, fmt.Sprintf("should not be present: %s", k))
			}
		}
	}
	assert.Equal(t, 3, tlsFieldsCount)
}
