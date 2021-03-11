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

package interceptors

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

func TestMetrics(t *testing.T) {
	registry := monitoring.NewRegistry()
	monitoringMap := request.MonitoringMapForRegistry(registry, request.DefaultResultIDs)
	methodName := "test_method_name"

	testMap := map[string]map[request.ResultID]*monitoring.Int{
		methodName: monitoringMap,
	}
	i := Metrics(testMap)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: methodName,
	}

	for _, tc := range []struct {
		f             func(ctx context.Context, req interface{}) (interface{}, error)
		monitoringInt map[request.ResultID]int64
	}{
		{
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, errors.New("error")
			},
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:           1,
				request.IDResponseCount:          1,
				request.IDResponseValidCount:     0,
				request.IDResponseErrorsCount:    1,
				request.IDResponseErrorsInternal: 1,
			},
		},
		{
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, nil
			},
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:           1,
				request.IDResponseCount:          1,
				request.IDResponseValidCount:     1,
				request.IDResponseErrorsCount:    0,
				request.IDResponseErrorsInternal: 0,
			},
		},
	} {
		i(ctx, nil, info, tc.f)
		assertMonitoring(t, tc.monitoringInt, monitoringMap)
		beatertest.ClearRegistry(monitoringMap)
	}
}

func assertMonitoring(t *testing.T, expected map[request.ResultID]int64, actual map[request.ResultID]*monitoring.Int) {
	for _, k := range request.DefaultResultIDs {
		if val, ok := expected[k]; ok {
			assert.Equalf(t, val, actual[k].Get(), "%s mismatch", k)
		} else {
			assert.Zerof(t, actual[k].Get(), "%s mismatch", k)
		}
	}
}
