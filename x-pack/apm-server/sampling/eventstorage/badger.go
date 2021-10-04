// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package eventstorage

import (
	"github.com/dgraph-io/badger/v2"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/beats/v7/libbeat/logp"
)

const (
	defaultValueLogFileSize = 128 * 1024 * 1024
)

// OpenBadger creates or opens a Badger database with the specified location
// and value log file size. If the value log file size is <= 0, the default
// of 128MB will be used.
//
// NOTE(axw) only one badger.DB for a given storage directory may be open at any given time.
func OpenBadger(storageDir string, valueLogFileSize int64) (*badger.DB, error) {
	logger := logp.NewLogger(logs.Sampling)
	badgerOpts := badger.DefaultOptions(storageDir)
	badgerOpts.ValueLogFileSize = defaultValueLogFileSize
	if valueLogFileSize > 0 {
		badgerOpts.ValueLogFileSize = valueLogFileSize
	}
	badgerOpts.Logger = LogpAdaptor{Logger: logger}
	return badger.Open(badgerOpts)
}
