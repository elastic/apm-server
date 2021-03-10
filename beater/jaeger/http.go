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

package jaeger

import (
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger/model"
	converter "github.com/jaegertracing/jaeger/model/converter/thrift/jaeger"
	"github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"go.opentelemetry.io/collector/consumer"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/request"
)

const (
	apiTracesRoute = "/api/traces"
)

var (
	httpRegistry      = monitoring.Default.NewRegistry("apm-server.jaeger.http")
	httpMonitoringMap = request.MonitoringMapForRegistry(httpRegistry, interceptors.MetricsMonitoringKeys)
)

// newHTTPMux returns a new http.ServeMux which accepts Thrift-encoded spans.
func newHTTPMux(consumer consumer.TracesConsumer) (*http.ServeMux, error) {
	handler, err := middleware.Wrap(
		newHTTPHandler(consumer),
		middleware.LogMiddleware(),
		middleware.RecoverPanicMiddleware(),
		middleware.MonitoringMiddleware(httpMonitoringMap),
		middleware.RequestTimeMiddleware(),
	)
	if err != nil {
		return nil, err
	}

	pool := request.NewContextPool()
	mux := http.NewServeMux()
	mux.Handle(apiTracesRoute, pool.HTTPHandler(handler))
	return mux, nil
}

type httpHandler struct {
	consumer consumer.TracesConsumer
}

func newHTTPHandler(consumer consumer.TracesConsumer) request.Handler {
	h := &httpHandler{consumer}
	return h.handle
}

func (h *httpHandler) handle(c *request.Context) {
	switch c.Request.URL.Path {
	case apiTracesRoute:
		h.handleTraces(c)
	default:
		c.Result.SetWithError(request.IDResponseErrorsNotFound, errors.New("unknown route"))
	}
	c.Write()
}

func (h *httpHandler) handleTraces(c *request.Context) {
	if c.Request.Method != http.MethodPost {
		c.Result.SetWithError(
			request.IDResponseErrorsMethodNotAllowed,
			errors.New("only POST requests are allowed"),
		)
		return
	}

	contentType, _, err := mime.ParseMediaType(c.Request.Header.Get("Content-Type"))
	if err != nil {
		c.Result.SetWithError(request.IDResponseErrorsValidate, err)
		return
	}
	switch contentType {
	case "application/x-thrift", "application/vnd.apache.thrift.binary":
	default:
		c.Result.SetWithError(
			request.IDResponseErrorsValidate,
			fmt.Errorf("unsupported content-type %q", contentType),
		)
		return
	}

	var batch jaeger.Batch
	transport := thrift.NewStreamTransport(c.Request.Body, ioutil.Discard)
	protocol := thrift.NewTBinaryProtocolFactoryDefault().GetProtocol(transport)
	if err := batch.Read(protocol); err != nil {
		c.Result.SetWithError(request.IDResponseErrorsDecode, err)
		return
	}

	modelBatch := model.Batch{
		Process: converter.ToDomainProcess(batch.Process),
		Spans:   converter.ToDomain(batch.Spans, batch.Process),
	}
	if err := consumeBatch(c.Request.Context(), modelBatch, h.consumer, httpMonitoringMap); err != nil {
		// TODO(axw) map errors from the consumer back to appropriate error codes?
		c.Result.SetWithError(request.IDResponseErrorsInternal, err)
		return
	}
	c.Result.SetDefault(request.IDResponseValidAccepted)
}
