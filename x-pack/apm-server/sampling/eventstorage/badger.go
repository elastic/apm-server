package eventstorage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/elastic/apm-data/model/modelpb"
)

const (
	// NOTE(axw) these values (and their meanings) must remain stable
	// over time, to avoid misinterpreting historical data.
	badgerEntryMetaTraceSampled   = 's'
	badgerEntryMetaTraceUnsampled = 'u'
	badgerEntryMetaTraceEvent     = 'e'
)

type badgerStorage struct {
	db      *badger.DB
	codec   Codec
	enabled bool
	mu      sync.RWMutex
}

func newBadgerStorage(storageDir string, codec Codec) (*badgerStorage, error) {
	if _, err := os.Stat(filepath.Join(storageDir, "MANIFEST")); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	// not setting read only as it will fail to replay vlog if it crashed previously
	opts := badger.DefaultOptions(storageDir).WithCompactL0OnClose(false)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &badgerStorage{
		db:      db,
		codec:   codec,
		enabled: true,
	}, nil
}

func (s *badgerStorage) disable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = false
	return s.db.Close()
}

type BadgerMigrationRW struct {
	s      *badgerStorage
	nextRW RW
}

// WriteTraceSampled records the tail-sampling decision for the given trace ID.
func (rw *BadgerMigrationRW) WriteTraceSampled(traceID string, sampled bool) error {
	return rw.nextRW.WriteTraceSampled(traceID, sampled)
}

// IsTraceSampled reports whether traceID belongs to a trace that is sampled
// or unsampled. If no sampling decision has been recorded, IsTraceSampled
// returns ErrNotFound.
func (rw *BadgerMigrationRW) IsTraceSampled(traceID string) (bool, error) {
	sampled, err := rw.nextRW.IsTraceSampled(traceID)
	if err != ErrNotFound {
		return sampled, err
	}

	if rw.s == nil {
		return sampled, err
	}
	rw.s.mu.RLock()
	defer rw.s.mu.RUnlock()
	if !rw.s.enabled {
		return sampled, err
	}

	txn := rw.s.db.NewTransaction(false)
	defer txn.Discard()
	item, err := txn.Get([]byte(traceID))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return false, ErrNotFound
		}
		return false, err
	}
	return item.UserMeta() == badgerEntryMetaTraceSampled, nil
}

func (rw *BadgerMigrationRW) WriteTraceEvent(traceID string, id string, event *modelpb.APMEvent) error {
	return rw.nextRW.WriteTraceEvent(traceID, id, event)
}

// DeleteTraceEvent deletes the trace event from storage.
func (rw *BadgerMigrationRW) DeleteTraceEvent(traceID, id string) error {
	// FIXME: inclined to not delete from badger.
	// At worst it will produce duplicates if apm-server restarts without persisting pubsub checkpoint.
	return rw.nextRW.DeleteTraceEvent(traceID, id)
}

// ReadTraceEvents reads trace events with the given trace ID from storage into out.
func (rw *BadgerMigrationRW) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	err := rw.nextRW.ReadTraceEvents(traceID, out)

	if rw.s == nil {
		return err
	}
	rw.s.mu.RLock()
	defer rw.s.mu.RUnlock()
	if !rw.s.enabled {
		return err
	}

	txn := rw.s.db.NewTransaction(false)
	opts := badger.DefaultIteratorOptions
	opts.Prefix = append([]byte(traceID), ':')

	// 1st pass: check whether there exist keys matching the prefix.
	// Do not prefetch values so that the check is done in-memory.
	// This is to optimize for cases when it is a miss.
	opts.PrefetchValues = false
	iter := txn.NewIterator(opts)
	iter.Rewind()
	if !iter.Valid() {
		iter.Close()
		return nil
	}
	iter.Close()

	// 2nd pass: this is only done when there exist keys matching the prefix.
	// Fetch the events with PrefetchValues for performance.
	// This is to optimize for cases when it is a hit.
	opts.PrefetchValues = true
	iter = txn.NewIterator(opts)
	defer iter.Close()
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		if item.IsDeletedOrExpired() {
			continue
		}
		switch item.UserMeta() {
		case badgerEntryMetaTraceEvent:
			event := &modelpb.APMEvent{}
			if err := item.Value(func(data []byte) error {
				if err := rw.s.codec.DecodeEvent(data, event); err != nil {
					return fmt.Errorf("codec failed to decode event: %w", err)
				}
				return nil
			}); err != nil {
				return err
			}
			*out = append(*out, event)
		default:
			// Unknown entry meta: ignore.
			continue
		}
	}
	return nil
}
