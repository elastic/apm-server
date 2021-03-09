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

package middleware

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

// LogGRPC wraps implementations of gRPC methods to handle logging processing.
func LogGRPC(ctx context.Context, logger *logp.Logger, f func() error) error {
	var (
		err   error
		start = time.Now()
	)

	defer func() {
		logger = logger.With("@timestamp", start, "event.duration", time.Since(start))
		if err != nil {
			// TODO: Is this the code we want?
			logger.With("grpc.response.status_code", codes.Internal).Error(logp.Error(err))
			return
		}
		logger.With("grpc.response.status_code", codes.OK).Info("request ok")
	}()

	if err = f(); err != nil {
		return err
	}
	return nil
}

// MetricsGRPC increments the counters in map m according to the passed in
// error, the result of a gRPC method call.
func MetricsGRPC(m map[request.ResultID]*monitoring.Int, err error) error {
	responseID := request.IDResponseValidCount
	if err != nil {
		responseID = request.IDResponseErrorsCount
	}

	m[request.IDRequestCount].Inc()
	m[request.IDResponseCount].Inc()
	m[responseID].Inc()

	return err
}
