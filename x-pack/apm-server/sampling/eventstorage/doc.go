// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package eventstorage implements the storage layer for tail-based sampling event
// and sampling decision read-writes.
//
// The database of choice is Pebble, which does not have TTL handling built-in,
// and we implement our own TTL handling on top of the database:
//   - TTL is divided up into N parts, where N is partitionsPerTTL.
//   - A database holds N + 1 + 1 partitions: N+1 active, and one inactive.
//   - Every TTL/N, the ring advances and the oldest active partition becomes inactive.
//   - Writes go to the current partition. Reads scan the N+1 active partitions.
//
// A read or write pins the partitions it uses and releases the partition lock
// before doing I/O, so a rotation never blocks readers behind a slow one.
// A partition is deleted only after its pins drop to zero: the ring advances,
// the new current partition is persisted, the rotation waits for pins on the
// outgoing partition, and only then deletes it.
//
// The new current partition is persisted before the wait so that a crash during
// the wait restarts on the partition already receiving writes. Restarting on the
// old current would make reads skip those writes, as that partition would be
// inactive, until it becomes current again.
//
// Waiting for the outgoing partition from inside an iteration over the active
// partitions deadlocks, since the iteration holds a pin on it.
package eventstorage
