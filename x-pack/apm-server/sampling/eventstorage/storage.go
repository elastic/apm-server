// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"errors"
	"io"

	"github.com/cockroachdb/pebble/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	// NOTE(axw) these values (and their meanings) must remain stable
	// over time, to avoid misinterpreting historical data.
	entryMetaTraceSampled   byte = 's'
	entryMetaTraceUnsampled byte = 'u'
)

var (
	// ErrNotFound is returned by by the Storage.IsTraceSampled method,
	// for non-existing trace IDs.
	ErrNotFound = errors.New("key not found")

	// ErrLimitReached is returned by the ReadWriter.Flush method when
	// the configured StorageLimiter.Limit is true.
	ErrLimitReached = errors.New("configured storage limit reached")
)

type db interface {
	Get(key []byte) ([]byte, io.Closer, error)
	Set(key, value []byte, opts *pebble.WriteOptions) error
	Delete(key []byte, opts *pebble.WriteOptions) error
	NewIter(o *pebble.IterOptions) (*pebble.Iterator, error)
}

type partitionedDB interface {
	db
	ReadPartitions() PartitionIterator
	WritePartition() PartitionIterator
}

// Storage provides storage for sampled transactions and spans,
// and for recording trace sampling decisions.
type Storage struct {
	db    partitionedDB
	codec Codec
}

// Codec provides methods for encoding and decoding events.
type Codec interface {
	DecodeEvent([]byte, *modelpb.APMEvent) error
	EncodeEvent(*modelpb.APMEvent) ([]byte, error)
}

// New returns a new Storage using db and codec.
func New(db partitionedDB, codec Codec) *Storage {
	return &Storage{
		db:    db,
		codec: codec,
	}
}

// NewReadWriter returns a new PartitionReadWriter for reading events from and
// writing events to storage.
//
// The returned PartitionReadWriter must be closed when it is no longer needed.
func (s *Storage) NewReadWriter() *PartitionReadWriter {
	return &PartitionReadWriter{
		s: s,
	}
}

// WriterOpts provides configuration options for writes to storage
type WriterOpts struct {
	StorageLimitInBytes int64
}
