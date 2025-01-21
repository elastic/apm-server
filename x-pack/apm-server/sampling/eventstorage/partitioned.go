package eventstorage

import (
	"bytes"
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/elastic/apm-data/model/modelpb"
)

func NewPartitionedReadWriter(db db, partitionID int32, codec Codec) PartitionedReadWriter {
	return PartitionedReadWriter{db: db, partitionID: partitionID, codec: codec}
}

type PartitionedReadWriter struct {
	db          db
	partitionID int32
	codec       Codec
}

func (rw PartitionedReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	var lb bytes.Buffer
	lb.Grow(1 + 1 + len(traceID) + 1)
	lb.WriteByte(byte(rw.partitionID))
	lb.WriteByte('@')
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

func (rw PartitionedReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	data, err := rw.codec.EncodeEvent(event)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	b.Grow(1 + 1 + len(traceID) + 1 + len(id))
	b.WriteByte(byte(rw.partitionID))
	b.WriteByte('@')
	b.WriteString(traceID)
	b.WriteByte(':')
	b.WriteString(id)
	key := b.Bytes()
	return rw.writeEntry(key, data)
}

func (rw *PartitionedReadWriter) writeEntry(key, data []byte) error {
	if err := rw.db.Set(key, data, pebble.NoSync); err != nil {
		return err
	}
	return nil
}

func (rw PartitionedReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	var b bytes.Buffer
	b.Grow(1 + 1 + len(traceID))
	b.WriteByte(byte(rw.partitionID))
	b.WriteByte('@')
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

func (rw PartitionedReadWriter) IsTraceSampled(traceID string) (bool, error) {
	var b bytes.Buffer
	b.Grow(1 + 1 + len(traceID))
	b.WriteByte(byte(rw.partitionID))
	b.WriteByte('@')
	b.WriteString(traceID)

	item, closer, err := rw.db.Get(b.Bytes())
	if err == pebble.ErrNotFound {
		return false, ErrNotFound
	}
	defer closer.Close()
	return item[0] == entryMetaTraceSampled, nil
}

func (rw PartitionedReadWriter) DeleteTraceEvent(traceID, id string) error {
	var b bytes.Buffer
	b.Grow(1 + 1 + len(traceID) + 1 + len(id))
	b.WriteByte(byte(rw.partitionID))
	b.WriteByte('@')
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

func (rw PartitionedReadWriter) Flush() error {
	//TODO implement me
	panic("implement me")
}
