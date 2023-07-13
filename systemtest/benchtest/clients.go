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
	"net/url"
	"testing"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/elastic/apm-perf/loadgen"
	loadgencfg "github.com/elastic/apm-perf/loadgen/config"
	"github.com/elastic/apm-perf/loadgen/eventhandler"

	"go.elastic.co/apm/v2"
	"go.elastic.co/apm/v2/transport"
)

// NewTracer returns a new Elastic APM tracer, configured
// to send to the target APM Server.
func NewTracer(tb testing.TB) *apm.Tracer {
	httpTransport, err := transport.NewHTTPTransport(transport.HTTPTransportOptions{
		ServerURLs:  []*url.URL{loadgencfg.Config.ServerURL},
		SecretToken: loadgencfg.Config.SecretToken,
	})
	if err != nil {
		tb.Fatal(err)
	}
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
func NewOTLPExporter(tb testing.TB) *otlptrace.Exporter {
	serverURL := loadgencfg.Config.ServerURL
	secretToken := loadgencfg.Config.SecretToken
	endpoint := serverURL.Host
	if serverURL.Port() == "" {
		switch serverURL.Scheme {
		case "http":
			endpoint += ":80"
		case "https":
			endpoint += ":443"
		}
	}
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(grpc.WithBlock()),
	}
	if secretToken != "" {
		opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{
			"Authorization": "Bearer " + secretToken,
		}))
	}
	if serverURL.Scheme == "http" {
		opts = append(opts, otlptracegrpc.WithInsecure())
	} else {
		tlsCredentials := credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true,
		})
		opts = append(opts, otlptracegrpc.WithTLSCredentials(tlsCredentials))
	}
	exporter, err := otlptracegrpc.New(context.Background(), opts...)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { exporter.Shutdown(context.Background()) })
	return exporter
}

// NewEventHandler creates a eventhandler which loads the files matching the
// passed regex.
func NewEventHandler(tb testing.TB, p string, l *rate.Limiter) *eventhandler.Handler {
	serverCfg := loadgencfg.Config
	h, err := loadgen.NewEventHandler(loadgen.EventHandlerParams{
		Path:              p,
		URL:               serverCfg.ServerURL.String(),
		Token:             serverCfg.SecretToken,
		Limiter:           l,
		RewriteIDs:        serverCfg.RewriteIDs,
		RewriteTimestamps: serverCfg.RewriteTimestamps,
		Headers:           serverCfg.Headers,
	})
	if err != nil {
		tb.Fatal(err)
	}
	return h
}
