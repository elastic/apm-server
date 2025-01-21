package eventstorage

import (
	"bytes"
	"time"

	"github.com/cockroachdb/pebble"

	"github.com/elastic/apm-data/model/modelpb"
)

func NewTTLReadWriter(truncatedTime time.Time, db db) TTLReadWriter {
	return TTLReadWriter{truncatedTime: truncatedTime, db: db}
}

type TTLReadWriter struct {
	truncatedTime time.Time
	db            db
}

func (rw TTLReadWriter) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	panic("implement me")
}

func (rw TTLReadWriter) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error {
	//TODO implement me
	panic("implement me")
}

func (rw TTLReadWriter) WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error {
	tp := timePrefix(rw.truncatedTime)

	var b bytes.Buffer
	b.Grow(len(tp) + 1 + len(traceID))
	b.WriteString(tp)
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

func (rw TTLReadWriter) IsTraceSampled(traceID string) (bool, error) {
	tp := timePrefix(rw.truncatedTime)

	var b bytes.Buffer
	b.Grow(len(tp) + 1 + len(traceID))
	b.WriteString(tp)
	b.WriteByte('@')
	b.WriteString(traceID)

	item, closer, err := rw.db.Get(b.Bytes())
	if err == pebble.ErrNotFound {
		return false, ErrNotFound
	}
	defer closer.Close()
	return item[0] == entryMetaTraceSampled, nil
}

func (rw TTLReadWriter) DeleteTraceEvent(traceID, id string) error {
	//TODO implement me
	panic("implement me")
}

func (rw TTLReadWriter) Flush() error {
	//TODO implement me
	panic("implement me")
}
