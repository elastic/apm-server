// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import "github.com/cockroachdb/pebble/v2/vfs"

type DiskThresholdChecker struct {
	path      string
	threshold float64
}

func NewDiskThresholdChecker(path string, threshold float64) DiskThresholdChecker {
	return DiskThresholdChecker{
		path:      path,
		threshold: threshold,
	}
}

func (d DiskThresholdChecker) DiskUsage() uint64 {
	usage, err := vfs.Default.GetDiskUsage(d.path) // FIXME: cache results
	if err != nil {
		panic(err) // FIXME: handle err
	}
	return usage.UsedBytes
}

func (d DiskThresholdChecker) StorageLimit() uint64 {
	usage, err := vfs.Default.GetDiskUsage(d.path) // FIXME: cache results
	if err != nil {
		panic(err) // FIXME: handle err
	}
	return uint64(float64(usage.TotalBytes) * d.threshold)
}
