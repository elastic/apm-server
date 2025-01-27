// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"io"

	"github.com/cockroachdb/pebble/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

type db interface {
	Get(key []byte) ([]byte, io.Closer, error)
	Set(key, value []byte, opts *pebble.WriteOptions) error
	Delete(key []byte, opts *pebble.WriteOptions) error
	NewIter(o *pebble.IterOptions) (*pebble.Iterator, error)
}

// Storage provides storage for sampled transactions and spans,
// and for recording trace sampling decisions.
type Storage struct {
	db          db
	partitioner *Partitioner
	codec       Codec
}

// Codec provides methods for encoding and decoding events.
type Codec interface {
	DecodeEvent([]byte, *modelpb.APMEvent) error
	EncodeEvent(*modelpb.APMEvent) ([]byte, error)
}

// New returns a new Storage using db and codec.
func New(db db, partitioner *Partitioner, codec Codec) *Storage {
	return &Storage{
		db:          db,
		partitioner: partitioner,
		codec:       codec,
	}
}

// NewReadWriter returns a new PartitionReadWriter for reading events from and
// writing events to storage.
func (s *Storage) NewReadWriter() *PartitionReadWriter {
	return &PartitionReadWriter{
		s: s,
	}
}
