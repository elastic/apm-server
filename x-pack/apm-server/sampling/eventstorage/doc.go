// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package eventstorage implements the storage layer for tail-based sampling event
// and sampling decision read-writes.
//
// The database of choice is Pebble, which does not have TTL handling built-in,
// and we implement our own TTL handling on top of the database:
// - TTL is divided up into N parts, where N is partitionsPerTTL.
// - A database holds N + 1 + 1 partitions.
// - Every TTL/N we will discard the oldest partition, so we keep a rolling window of N+1 partitions.
// - Writes will go to the most recent partition, and we'll read across N+1 partitions
package eventstorage
