// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"github.com/elastic/apm-data/model/modelpb"
)

type RW interface {
	ReadTraceEvents(traceID string, out *modelpb.Batch) error
	WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error
	WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error
	IsTraceSampled(traceID string) (bool, error)
	DeleteTraceEvent(traceID, id string) error
	Flush() error
}
