package eventstorage

import (
	"bytes"

	"github.com/cockroachdb/pebble"

	"github.com/elastic/apm-data/model/modelpb"
)

func NewPartitionedReadWriter(db db, partitionID int32) PartitionedReadWriter {
	return PartitionedReadWriter{db: db, partitionID: partitionID}
}

type PartitionedReadWriter struct {
	db          db
	partitionID int32
}

func (rw PartitionedReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	panic("implement me")
}

func (rw PartitionedReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	//TODO implement me
	panic("implement me")
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
	//TODO implement me
	panic("implement me")
}

func (rw PartitionedReadWriter) Flush() error {
	//TODO implement me
	panic("implement me")
}
