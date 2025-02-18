// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"bytes"
	"path/filepath"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/bloom"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

func eventComparer() *pebble.Comparer {
	comparer := *pebble.DefaultComparer
	// Required for prefix bloom filter
	comparer.Split = func(k []byte) int {
		if idx := bytes.IndexByte(k, traceIDSeparator); idx != -1 {
			return idx + 1
		}
		// If traceID separator does not exist, consider the entire key as prefix.
		// This is required for deletes like DeleteRange([]byte{0}, []byte{1}) to work without specifying the separator.
		return len(k)
	}
	comparer.Compare = func(a, b []byte) int {
		ap := comparer.Split(a) // a prefix length
		bp := comparer.Split(b) // b prefix length
		if prefixCmp := bytes.Compare(a[:ap], b[:bp]); prefixCmp != 0 {
			return prefixCmp
		}
		return comparer.ComparePointSuffixes(a[ap:], b[bp:])
	}
	comparer.Name = "apmserver.EventComparer" // this should stay constant, otherwise existing database won't open
	return &comparer
}

func OpenEventPebble(storageDir string) (*pebble.DB, error) {
	// Option values are picked and validated in https://github.com/elastic/apm-server/issues/15568
	opts := &pebble.Options{
		FormatMajorVersion: pebble.FormatColumnarBlocks,
		Logger:             logp.NewLogger(logs.Sampling),
		MemTableSize:       16 << 20,
		Levels: []pebble.LevelOptions{
			{
				BlockSize:    32 << 10, // the bigger the blocks, the better the compression and the smaller the index block
				Compression:  func() pebble.Compression { return pebble.SnappyCompression },
				FilterPolicy: bloom.FilterPolicy(10),
				FilterType:   pebble.TableFilter,
			},
		},
		Comparer: eventComparer(),
	}
	return pebble.Open(filepath.Join(storageDir, "event"), opts)
}

func OpenDecisionPebble(storageDir string) (*pebble.DB, error) {
	// Option values are picked and validated in https://github.com/elastic/apm-server/issues/15568
	return pebble.Open(filepath.Join(storageDir, "decision"), &pebble.Options{
		FormatMajorVersion: pebble.FormatColumnarBlocks,
		Logger:             logp.NewLogger(logs.Sampling),
		MemTableSize:       2 << 20, // big memtables are slow to scan, and significantly slow the hot path
		Levels: []pebble.LevelOptions{
			{
				BlockSize:    2 << 10,
				Compression:  func() pebble.Compression { return pebble.NoCompression },
				FilterPolicy: bloom.FilterPolicy(10),
				FilterType:   pebble.TableFilter,
			},
		},
	})
}
