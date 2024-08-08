// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type config struct {
	Meter metric.Meter
}

// Option interface is used to configure optional config options.
type Option interface {
	apply(*config)
}

type optionFunc func(*config)

func (o optionFunc) apply(c *config) {
	o(c)
}

func newConfig(opts ...Option) *config {
	c := &config{
		Meter: otel.GetMeterProvider().Meter("aggregators"),
	}
	for _, opt := range opts {
		opt.apply(c)
	}
	return c
}

// WithMeter configures a meter to use for telemetry. If no meter is
// passed then the meter is created using the global provider.
func WithMeter(meter metric.Meter) Option {
	return optionFunc(func(cfg *config) {
		cfg.Meter = meter
	})
}
