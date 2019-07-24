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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
)

func TestMonitoringHandler(t *testing.T) {
	t.Run("Error", func(t *testing.T) {
		beatertest.ClearRegistry(mockMonitoringRegistry, mockMonitoringFn)
		c, _ := beatertest.DefaultContextWithResponseRecorder()
		WithMiddleware(beatertest.Handler403, MonitoringHandler(mockMonitoringFn))(c)
		expected := map[request.ResultID]int{
			request.IDRequestCount:            1,
			request.IDResponseCount:           1,
			request.IDResponseErrorsCount:     1,
			request.IDResponseErrorsForbidden: 1}
		equal, result := beatertest.CompareMonitoringInt(expected, mockMonitoringFn)
		assert.True(t, equal, result)
	})
	t.Run("Accepted", func(t *testing.T) {
		beatertest.ClearRegistry(mockMonitoringRegistry, mockMonitoringFn)
		c, _ := beatertest.DefaultContextWithResponseRecorder()
		WithMiddleware(beatertest.Handler202, MonitoringHandler(mockMonitoringFn))(c)
		expected := map[request.ResultID]int{
			request.IDRequestCount:          1,
			request.IDResponseCount:         1,
			request.IDResponseValidCount:    1,
			request.IDResponseValidAccepted: 1}
		equal, result := beatertest.CompareMonitoringInt(expected, mockMonitoringFn)
		assert.True(t, equal, result)
	})
	t.Run("Idle", func(t *testing.T) {
		beatertest.ClearRegistry(mockMonitoringRegistry, mockMonitoringFn)
		c, _ := beatertest.DefaultContextWithResponseRecorder()
		WithMiddleware(beatertest.HandlerIdle, MonitoringHandler(mockMonitoringFn))(c)
		expected := map[request.ResultID]int{
			request.IDRequestCount:       1,
			request.IDResponseCount:      1,
			request.IDResponseValidCount: 1,
			request.IDUnset:              1}
		equal, result := beatertest.CompareMonitoringInt(expected, mockMonitoringFn)
		assert.True(t, equal, result)
	})
}

var (
	mockMonitoringRegistry = monitoring.Default.NewRegistry("mock.monitoring", monitoring.PublishExpvar)
	mockMonitoringMap      = map[request.ResultID]*monitoring.Int{}
	mockCounterFn          = func(s string) *monitoring.Int {
		return monitoring.NewInt(mockMonitoringRegistry, s)
	}
)

func mockMonitoringFn(name request.ResultID) *monitoring.Int {
	if i, ok := mockMonitoringMap[name]; ok {
		return i
	}
	ct := mockCounterFn(string(name))
	mockMonitoringMap[name] = ct
	return ct
}
