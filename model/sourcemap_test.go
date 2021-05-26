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

package model_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"

	s "github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/elasticsearch/estest"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
)

func getStr(data common.MapStr, key string) string {
	rs, _ := data.GetValue(key)
	return rs.(string)
}

func TestTransform(t *testing.T) {
	p := model.Sourcemap{
		ServiceName:    "myService",
		ServiceVersion: "1.0",
		BundleFilepath: "/my/path",
		Sourcemap:      "mysmap",
	}

	events := p.Transform(context.Background(), &transform.Config{})
	assert.Len(t, events, 1)
	event := events[0]

	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
	output := event.Fields["sourcemap"].(common.MapStr)

	assert.Equal(t, "/my/path", getStr(output, "bundle_filepath"))
	assert.Equal(t, "myService", getStr(output, "service.name"))
	assert.Equal(t, "1.0", getStr(output, "service.version"))
	assert.Equal(t, "mysmap", getStr(output, "sourcemap"))
}

func TestParseSourcemaps(t *testing.T) {
	fileBytes, err := ioutil.ReadFile("../testdata/sourcemap/bundle.js.map")
	assert.NoError(t, err)
	parser, err := s.Parse("", fileBytes)
	assert.NoError(t, err)

	source, _, _, _, ok := parser.Source(1, 9)
	assert.True(t, ok)
	assert.Equal(t, "webpack:///bundle.js", source)
}

func TestInvalidateCache(t *testing.T) {
	event := model.Sourcemap{ServiceName: "service", ServiceVersion: "1", BundleFilepath: "js/bundle.js", Sourcemap: "testmap"}
	t.Run("withSourcemapStore", func(t *testing.T) {
		// collect logs
		require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))

		// create sourcemap store
		client, err := estest.NewElasticsearchClient(estest.NewTransport(t, http.StatusOK, nil))
		require.NoError(t, err)
		store, err := sourcemap.NewStore(client, "foo", time.Minute)
		require.NoError(t, err)

		// transform with sourcemap store
		event.Transform(context.Background(), &transform.Config{RUM: transform.RUMConfig{SourcemapStore: store}})

		logCollection := logp.ObserverLogs().TakeAll()
		assert.Equal(t, 2, len(logCollection))

		// first sourcemap was added
		for i, entry := range logCollection {
			assert.Equal(t, logs.Sourcemap, entry.LoggerName)
			assert.Equal(t, zapcore.DebugLevel, entry.Level)
			if i == 0 {
				assert.Contains(t, entry.Message, "Added id service_1_js/bundle.js. Cache now has 1 entries.")
			} else {
				assert.Contains(t, entry.Message, "Removed id service_1_js/bundle.js. Cache now has 0 entries.")
			}
		}

	})

	t.Run("noSourcemapStore", func(t *testing.T) {
		// collect logs
		require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))

		// transform with no sourcemap store
		event.Transform(context.Background(), &transform.Config{RUM: transform.RUMConfig{}})

		logCollection := logp.ObserverLogs().TakeAll()
		assert.Equal(t, 1, len(logCollection))
		for _, entry := range logCollection {
			assert.Equal(t, logs.Sourcemap, entry.LoggerName)
			assert.Equal(t, zapcore.ErrorLevel, entry.Level)
			assert.Contains(t, entry.Message, "cache cannot be invalidated")
		}

	})
}
