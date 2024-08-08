// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// otelKVsToZapFields converts []attribute.KeyValue to []zap.Field.
// Designed to work with CombinedMetricsIDToKVs for logging.
func otelKVsToZapFields(kvs []attribute.KeyValue) []zap.Field {
	if kvs == nil {
		return nil
	}
	fields := make([]zap.Field, len(kvs))
	for i, kv := range kvs {
		fields[i] = zap.Any(string(kv.Key), kv.Value.AsInterface())
	}
	return fields
}
