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

package benchtest

import (
	"context"
	"crypto/tls"
	"testing"

	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.elastic.co/apm"
	"go.elastic.co/apm/transport"
)

func init() {
	// Close default tracer, we'll create new ones.
	apm.DefaultTracer.Close()
}

// NewTracer returns a new Elastic APM tracer, configured
// to send to the target APM Server.
func NewTracer(tb testing.TB) *apm.Tracer {
	httpTransport, err := transport.NewHTTPTransport()
	if err != nil {
		tb.Fatal(err)
	}
	httpTransport.SetServerURL(serverURL)
	httpTransport.SetSecretToken(*secretToken)
	tracer, err := apm.NewTracerOptions(apm.TracerOptions{
		Transport: httpTransport,
	})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(tracer.Close)
	return tracer
}

// NewOTLPExporter returns a new OpenTelemetry Go exporter, configured
// to export to the target APM Server.
func NewOTLPExporter(tb testing.TB) *otlp.Exporter {
	endpoint := serverURL.Host
	if serverURL.Port() == "" {
		switch serverURL.Scheme {
		case "http":
			endpoint += ":80"
		case "https":
			endpoint += ":443"
		}
	}
	opts := []otlpgrpc.Option{
		otlpgrpc.WithEndpoint(endpoint),
		otlpgrpc.WithDialOption(grpc.WithBlock()),
	}
	if *secretToken != "" {
		opts = append(opts, otlpgrpc.WithHeaders(map[string]string{
			"Authorization": "Bearer " + *secretToken,
		}))
	}
	if serverURL.Scheme == "http" {
		opts = append(opts, otlpgrpc.WithInsecure())
	} else {
		tlsCredentials := credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true,
		})
		opts = append(opts, otlpgrpc.WithTLSCredentials(tlsCredentials))
	}
	exporter, err := otlp.NewExporter(context.Background(), otlpgrpc.NewDriver(opts...))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { exporter.Shutdown(context.Background()) })
	return exporter
}
