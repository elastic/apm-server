// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	defaultValueLogFileSize = 64 * 1024 * 1024
)

// OpenBadger creates or opens a Badger database with the specified location
// and value log file size. If the value log file size is <= 0, the default
// of 64MB will be used.
//
// NOTE(axw) only one badger.DB for a given storage directory may be open at any given time.
func OpenBadger(storageDir string, valueLogFileSize int64) (*badger.DB, error) {
	logger := logp.NewLogger(logs.Sampling)
	// Tunable memory options:
	//  - NumMemtables - default 5 in-mem tables (MaxTableSize default)
	//  - NumLevelZeroTables - default 5 - number of L0 tables before compaction starts.
	//  - NumLevelZeroTablesStall - number of L0 tables before writing stalls (waiting for compaction).
	//  - IndexCacheSize - default all in mem, Each table has its own bloom filter and each bloom filter is approximately of 5 MB.
	//  - MaxTableSize - Default 64MB
	if valueLogFileSize <= 0 {
		valueLogFileSize = defaultValueLogFileSize
	}
	const tableLimit = 4
	badgerOpts := badger.DefaultOptions(storageDir).
		WithLogger(&LogpAdaptor{Logger: logger}).
		WithTruncate(true).                          // Truncate unreadable files which cannot be read.
		WithNumMemtables(tableLimit).                // in-memory tables.
		WithNumLevelZeroTables(tableLimit).          // L0 tables.
		WithNumLevelZeroTablesStall(tableLimit * 3). // Maintain the default 1-to-3 ratio before stalling.
		WithMaxTableSize(int64(16 << 20)).           // Max LSM table or file size.
		WithValueLogFileSize(valueLogFileSize)       // vlog file size.

	return badger.Open(badgerOpts)
}
