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
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	elasticapm "github.com/elastic/apm-agent-go"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

func TestNotifyUpServerDown(t *testing.T) {
	config := defaultConfig("7.0.0")
	var saved beat.Event
	var publisher = func(e beat.Event) { saved = e }

	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)
	defer lis.Close()
	config.Host = lis.Addr().String()

	server, err := newServer(config, elasticapm.DefaultTracer, nopReporter)
	require.NoError(t, err)
	go run(server, lis, config)

	notifyListening(config, publisher)

	listening := saved.Fields["listening"].(string)
	assert.Equal(t, config.Host, listening)

	processor := saved.Fields["processor"].(common.MapStr)
	assert.Equal(t, "onboarding", processor["name"])
	assert.Equal(t, "onboarding", processor["event"])

}
