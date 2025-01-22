// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"bytes"
	"fmt"

	"github.com/cockroachdb/pebble/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

func NewPrefixReadWriter(db db, prefix byte, codec Codec) PrefixReadWriter {
	return PrefixReadWriter{db: db, prefix: prefix, codec: codec}
}

type PrefixReadWriter struct {
	db     db
	prefix byte
	codec  Codec
}

func (rw PrefixReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	var lb bytes.Buffer
	lb.Grow(1 + len(traceID) + 1)
	lb.WriteByte(rw.prefix)
	lb.WriteString(traceID)
	lb.WriteByte(':')

	var ub bytes.Buffer
	ub.Grow(lb.Len())
	ub.Write(lb.Bytes()[:lb.Len()-1])
	ub.WriteByte(';') // This is a hack to stop before next ID

	iter, err := rw.db.NewIter(&pebble.IterOptions{
		LowerBound: lb.Bytes(),
		UpperBound: ub.Bytes(),
	})
	if err != nil {
		return err
	}
	defer iter.Close()
	// SeekPrefixGE uses the prefix bloom filter, so that a miss will be much faster
	if valid := iter.SeekPrefixGE(lb.Bytes()); !valid {
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

func (rw PrefixReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	data, err := rw.codec.EncodeEvent(event)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	b.Grow(1 + len(traceID) + 1 + len(id))
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)
	b.WriteByte(':')
	b.WriteString(id)
	key := b.Bytes()
	return rw.writeEntry(key, data)
}

func (rw PrefixReadWriter) writeEntry(key, data []byte) error {
	if err := rw.db.Set(key, data, pebble.NoSync); err != nil {
		return err
	}
	return nil
}

func (rw PrefixReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	var b bytes.Buffer
	b.Grow(1 + len(traceID))
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)

	meta := entryMetaTraceUnsampled
	if sampled {
		meta = entryMetaTraceSampled
	}
	err := rw.db.Set(b.Bytes(), []byte{meta}, pebble.NoSync)
	if err != nil {
		return err
	}
	return nil
}

func (rw PrefixReadWriter) IsTraceSampled(traceID string) (bool, error) {
	var b bytes.Buffer
	b.Grow(1 + len(traceID))
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)

	item, closer, err := rw.db.Get(b.Bytes())
	if err == pebble.ErrNotFound {
		return false, ErrNotFound
	}
	defer closer.Close()
	return item[0] == entryMetaTraceSampled, nil
}

func (rw PrefixReadWriter) DeleteTraceEvent(traceID, id string) error {
	var b bytes.Buffer
	b.Grow(1 + len(traceID) + 1 + len(id))
	b.WriteByte(rw.prefix)
	b.WriteString(traceID)
	b.WriteByte(':')
	b.WriteString(id)
	key := b.Bytes()

	err := rw.db.Delete(key, pebble.NoSync)
	if err != nil {
		return err
	}
	return nil
}

func (rw PrefixReadWriter) Flush() error {
	return nil
}
