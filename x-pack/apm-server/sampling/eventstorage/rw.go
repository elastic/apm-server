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

type SplitReadWriter struct {
	eventRW, decisionRW RW
}

func (s SplitReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	return s.eventRW.ReadTraceEvents(traceID, out)
}

func (s SplitReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	return s.eventRW.WriteTraceEvent(traceID, id, event, opts)
}

func (s SplitReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	return s.decisionRW.WriteTraceSampled(traceID, sampled, opts)
}

func (s SplitReadWriter) IsTraceSampled(traceID string) (bool, error) {
	return s.decisionRW.IsTraceSampled(traceID)
}

func (s SplitReadWriter) DeleteTraceEvent(traceID, id string) error {
	return s.eventRW.DeleteTraceEvent(traceID, id)
}

func (s SplitReadWriter) Flush() error {
	return nil
}

func (s SplitReadWriter) Close() error {
	return nil
}

type storageLimitChecker interface {
	StorageLimitReached() bool
}

type StorageLimitReadWriter struct {
	checker storageLimitChecker
	nextRW  RW
}

func (s StorageLimitReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	return s.nextRW.ReadTraceEvents(traceID, out)
}

func (s StorageLimitReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	if s.checker.StorageLimitReached() {
		return ErrLimitReached
	}
	return s.nextRW.WriteTraceEvent(traceID, id, event, opts)
}

func (s StorageLimitReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	if s.checker.StorageLimitReached() {
		return ErrLimitReached
	}
	return s.nextRW.WriteTraceSampled(traceID, sampled, opts)
}

func (s StorageLimitReadWriter) IsTraceSampled(traceID string) (bool, error) {
	return s.nextRW.IsTraceSampled(traceID)
}

func (s StorageLimitReadWriter) DeleteTraceEvent(traceID, id string) error {
	// Technically DeleteTraceEvent writes, but it should have a net effect of reducing disk usage
	return s.nextRW.DeleteTraceEvent(traceID, id)
}

func (s StorageLimitReadWriter) Flush() error {
	return s.nextRW.Flush()
}

func (s StorageLimitReadWriter) Close() error {
	return nil
}
