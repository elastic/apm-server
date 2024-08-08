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

package modelprocessor

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// ConfigOption allows passing a functional option when creating a new
// processor
type ConfigOption func(config) config

// config allow processors to get optional arguments.
// The config struct is shared amongst every processor using it, but they may
// not each use every option.
type config struct {
	tracerProvider trace.TracerProvider
}

func newConfig(opts ...ConfigOption) config {
	config := config{}
	for _, opt := range opts {
		config = opt(config)
	}

	if config.tracerProvider == nil {
		config.tracerProvider = otel.GetTracerProvider()
	}

	return config
}

// WithTracerProvider allows setting a custom tracer provider
// Defaults to the global tracer provider
func WithTracerProvider(tp trace.TracerProvider) ConfigOption {
	return func(c config) config {
		c.tracerProvider = tp
		return c
	}
}
