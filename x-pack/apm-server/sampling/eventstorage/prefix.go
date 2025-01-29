// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	// NOTE(axw) these values (and their meanings) must remain stable
	// over time, to avoid misinterpreting historical data.
	entryMetaTraceSampled   byte = 's'
	entryMetaTraceUnsampled byte = 'u'

	// traceIDSeparator is the separator between trace ID and transaction / span ID
	traceIDSeparator byte = ':'
)

var (
	// ErrNotFound is returned by the RW.IsTraceSampled method,
	// for non-existing trace IDs.
	ErrNotFound = errors.New("key not found")

	// Reuse sampling decision value byte slices for performance
	traceSampledVal   = []byte{entryMetaTraceSampled}
	traceUnsampledVal = []byte{entryMetaTraceUnsampled}
)

func NewPrefixReadWriter(db db, prefix byte, codec Codec) PrefixReadWriter {
	return PrefixReadWriter{db: db, prefix: prefix, codec: codec}
}

// PrefixReadWriter is a helper read writer that reads from and writes to db with a prefix byte in key.
type PrefixReadWriter struct {
	db     db
	prefix byte
	codec  Codec
}

// ReadTraceEvents reads rw.db using a key consisting of rw.prefix and traceID, and appends results to out.
func (rw PrefixReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	var b bytes.Buffer
	b.Grow(1 + len(traceID) + 1)
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)
	b.WriteByte(traceIDSeparator)

	iter, err := rw.db.NewIter(&pebble.IterOptions{})
	if err != nil {
		return err
	}
	defer iter.Close()

	// SeekPrefixGE uses prefix bloom filter for on disk tables.
	// These bloom filters are cached in memory, and a "miss" on bloom filter avoids disk IO to check the actual table.
	// Memtables still need to be scanned as pebble has no bloom filter on memtables.
	//
	// SeekPrefixGE ensures the prefix is present and does not require lower bound and upper bound to be set on iterator.
	if valid := iter.SeekPrefixGE(b.Bytes()); !valid {
		return nil
	}
	for ; iter.Valid(); iter.Next() {
		event := &modelpb.APMEvent{}
		data, err := iter.ValueAndErr()
		if err != nil {
			return err
		}
		if err := rw.codec.DecodeEvent(data, event); err != nil {
			return fmt.Errorf("codec failed to decode event: %w", err)
		}
		*out = append(*out, event)
	}
	return nil
}

// WriteTraceEvent writes encoded event as value to rw.db with key consisting of rw.prefix, traceID and id.
func (rw PrefixReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent) error {
	data, err := rw.codec.EncodeEvent(event)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	b.Grow(1 + len(traceID) + 1 + len(id))
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)
	b.WriteByte(traceIDSeparator)
	b.WriteString(id)
	key := b.Bytes()
	return rw.db.Set(key, data, pebble.NoSync)
}

// WriteTraceSampled writes sampling decision sampled to rw.db with key consisting of rw.prefix and traceID.
func (rw PrefixReadWriter) WriteTraceSampled(traceID string, sampled bool) error {
	var b bytes.Buffer
	b.Grow(1 + len(traceID))
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)

	val := traceUnsampledVal
	if sampled {
		val = traceSampledVal
	}
	return rw.db.Set(b.Bytes(), val, pebble.NoSync)
}

// IsTraceSampled reads sampling decision from rw.db using a key consisting of rw.prefix and traceID.
// Returns ErrNotFound if trace sampling decision is not found.
func (rw PrefixReadWriter) IsTraceSampled(traceID string) (bool, error) {
	var b bytes.Buffer
	b.Grow(1 + len(traceID))
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)

	item, closer, err := rw.db.Get(b.Bytes())
	if err == pebble.ErrNotFound {
		return false, ErrNotFound
	} else if err != nil {
		return false, err
	}
	defer closer.Close()
	return item[0] == entryMetaTraceSampled, nil
}

// DeleteTraceEvent deletes event associated with key consisting of rw.prefix, traceID and id from rw.db.
func (rw PrefixReadWriter) DeleteTraceEvent(traceID, id string) error {
	var b bytes.Buffer
	b.Grow(1 + len(traceID) + 1 + len(id))
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)
	b.WriteByte(traceIDSeparator)
	b.WriteString(id)
	key := b.Bytes()

	return rw.db.Delete(key, pebble.NoSync)
}
