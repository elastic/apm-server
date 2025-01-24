// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"io"
	"iter"

	"github.com/cockroachdb/pebble/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

type db interface {
	Get(key []byte) ([]byte, io.Closer, error)
	Set(key, value []byte, opts *pebble.WriteOptions) error
	Delete(key []byte, opts *pebble.WriteOptions) error
	NewIter(o *pebble.IterOptions) (*pebble.Iterator, error)
}

type partitionedDB interface {
	db
	ReadPartitions() iter.Seq[int]
	WritePartition() int
}

type wrappedDB struct {
	partitioner *Partitioner
	db          *pebble.DB
}

func (w *wrappedDB) Get(key []byte) ([]byte, io.Closer, error) {
	return w.db.Get(key)
}

func (w *wrappedDB) Set(key, value []byte, opts *pebble.WriteOptions) error {
	return w.db.Set(key, value, opts)
}

func (w *wrappedDB) Delete(key []byte, opts *pebble.WriteOptions) error {
	return w.db.Delete(key, opts)
}

func (w *wrappedDB) NewIter(o *pebble.IterOptions) (*pebble.Iterator, error) {
	return w.db.NewIter(o)
}

// ReadPartitions returns ID of the partitions that all reads should read from.
// Reads should consider all active partitions as database entries may be written at
// any point of time in the past.
func (w *wrappedDB) ReadPartitions() iter.Seq[int] {
	return w.partitioner.Actives()
}

// WritePartition returns ID of the partition that current writes should write to.
func (w *wrappedDB) WritePartition() int {
	return w.partitioner.Current()
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
func (s *Storage) NewReadWriter() *PartitionReadWriter {
	return &PartitionReadWriter{
		s: s,
	}
}
