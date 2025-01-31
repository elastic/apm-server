// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"
	"fmt"

	"github.com/elastic/apm-data/model/modelpb"
)

var (
	// ErrLimitReached is returned by RW methods when storage usage
	// is greater than configured limit.
	ErrLimitReached = errors.New("configured limit reached")
)

// RW is a read writer interface that has methods to read and write trace event and sampling decisions.
type RW interface {
	ReadTraceEvents(traceID string, out *modelpb.Batch) error
	WriteTraceEvent(traceID, id string, event *modelpb.APMEvent) error
	WriteTraceSampled(traceID string, sampled bool) error
	IsTraceSampled(traceID string) (bool, error)
	DeleteTraceEvent(traceID, id string) error
}

// SplitReadWriter is a RW that splits method calls to eventRW and decisionRW.
// - *TraceEvent* method calls are passed through to eventRW.
// - *TraceSampled method calls are passed through to decisionRW.
type SplitReadWriter struct {
	eventRW, decisionRW RW
}

func (s SplitReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	return s.eventRW.ReadTraceEvents(traceID, out)
}

func (s SplitReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent) error {
	return s.eventRW.WriteTraceEvent(traceID, id, event)
}

func (s SplitReadWriter) WriteTraceSampled(traceID string, sampled bool) error {
	return s.decisionRW.WriteTraceSampled(traceID, sampled)
}

func (s SplitReadWriter) IsTraceSampled(traceID string) (bool, error) {
	return s.decisionRW.IsTraceSampled(traceID)
}

func (s SplitReadWriter) DeleteTraceEvent(traceID, id string) error {
	return s.eventRW.DeleteTraceEvent(traceID, id)
}

func (s SplitReadWriter) Close() error {
	return nil
}

type storageLimitChecker interface {
	DiskUsage() uint64
	StorageLimit() uint64
}

type storageLimitCheckerFunc struct {
	diskUsage, storageLimit func() uint64
}

func NewStorageLimitCheckerFunc(diskUsage, storageLimit func() uint64) storageLimitCheckerFunc {
	return storageLimitCheckerFunc{
		diskUsage:    diskUsage,
		storageLimit: storageLimit,
	}
}

func (f storageLimitCheckerFunc) DiskUsage() uint64 {
	return f.diskUsage()
}

func (f storageLimitCheckerFunc) StorageLimit() uint64 {
	return f.storageLimit()
}

// StorageLimitReadWriter is a RW that forbids Write* method calls based on disk usage and limit from storageLimitChecker.
// If there is no limit or limit is not reached, method calls are passed through to nextRW.
type StorageLimitReadWriter struct {
	name    string
	checker storageLimitChecker
	nextRW  RW
}

func NewStorageLimitReadWriter(name string, checker storageLimitChecker, nextRW RW) StorageLimitReadWriter {
	return StorageLimitReadWriter{
		name:    name,
		checker: checker,
		nextRW:  nextRW,
	}
}

func (s StorageLimitReadWriter) checkStorageLimit() error {
	limit := s.checker.StorageLimit()
	if limit != 0 { // unlimited storage
		usage := s.checker.DiskUsage()
		if usage >= limit {
			return fmt.Errorf("%s: %w (current: %d, limit: %d)", s.name, ErrLimitReached, usage, limit)
		}
	}
	return nil
}

// ReadTraceEvents passes through to s.nextRW.ReadTraceEvents.
func (s StorageLimitReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	return s.nextRW.ReadTraceEvents(traceID, out)
}

// WriteTraceEvent passes through to s.nextRW.WriteTraceEvent only if storage limit is not reached.
func (s StorageLimitReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent) error {
	if err := s.checkStorageLimit(); err != nil {
		return err
	}
	return s.nextRW.WriteTraceEvent(traceID, id, event)
}

// WriteTraceSampled passes through to s.nextRW.WriteTraceSampled only if storage limit is not reached.
func (s StorageLimitReadWriter) WriteTraceSampled(traceID string, sampled bool) error {
	if err := s.checkStorageLimit(); err != nil {
		return err
	}
	return s.nextRW.WriteTraceSampled(traceID, sampled)
}

// IsTraceSampled passes through to s.nextRW.IsTraceSampled.
func (s StorageLimitReadWriter) IsTraceSampled(traceID string) (bool, error) {
	return s.nextRW.IsTraceSampled(traceID)
}

// DeleteTraceEvent passes through to s.nextRW.DeleteTraceEvent.
func (s StorageLimitReadWriter) DeleteTraceEvent(traceID, id string) error {
	// Technically DeleteTraceEvent writes, but it should have a net effect of reducing disk usage
	return s.nextRW.DeleteTraceEvent(traceID, id)
}
