// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func BenchmarkWriteTransaction(b *testing.B) {
	test := func(b *testing.B, codec eventstorage.Codec, bigTX bool) {
		db := newBadgerDB(b, badgerOptions)
		store := eventstorage.New(db, codec)
		readWriter := store.NewReadWriter()
		defer readWriter.Close()

		traceID := hex.EncodeToString([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		transactionID := hex.EncodeToString([]byte{1, 2, 3, 4, 5, 6, 7, 8})
		var transaction *model.APMEvent
		if bigTX {
			transaction = makeTransaction(transactionID, traceID)
		} else {
			transaction = &model.APMEvent{
				Transaction: &model.Transaction{
					ID: transactionID,
				},
			}
		}

		b.ResetTimer()

		wOpts := eventstorage.WriterOpts{
			TTL:                 time.Minute,
			StorageLimitInBytes: 0,
		}
		for i := 0; i < b.N; i++ {
			if err := readWriter.WriteTraceEvent(traceID, transactionID, transaction, wOpts); err != nil {
				b.Fatal(err)
			}
		}
		assert.NoError(b, readWriter.Flush(wOpts.StorageLimitInBytes))
	}

	type testCase struct {
		codec eventstorage.Codec
		name  string
	}
	cases := []testCase{
		{
			name:  "json_codec",
			codec: eventstorage.JSONCodec{},
		},
		{
			// This tests the eventstorage performance without
			// JSON encoding. This would be the theoretical
			// upper limit of what we can achieve with a more
			// efficient codec.
			name:  "nop_codec",
			codec: nopCodec{},
		},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			test(b, tc.codec, false)
		})
		b.Run(tc.name+"_big_tx", func(b *testing.B) {
			test(b, tc.codec, true)
		})
	}
}

func BenchmarkReadEvents(b *testing.B) {
	traceID := uuid.Must(uuid.NewV4()).String()

	test := func(b *testing.B, codec eventstorage.Codec, bigTX bool) {
		// Test with varying numbers of events in the trace.
		counts := []int{0, 1, 10, 100, 199, 399, 1000}
		for _, count := range counts {
			b.Run(fmt.Sprintf("%d events", count), func(b *testing.B) {
				db := newBadgerDB(b, badgerOptions)
				store := eventstorage.New(db, codec)
				readWriter := store.NewReadWriter()
				defer readWriter.Close()
				wOpts := eventstorage.WriterOpts{
					TTL:                 time.Minute,
					StorageLimitInBytes: 0,
				}

				for i := 0; i < count; i++ {
					transactionID := uuid.Must(uuid.NewV4()).String()
					var transaction *model.APMEvent
					if bigTX {
						transaction = makeTransaction(transactionID, traceID)
					} else {
						transaction = &model.APMEvent{
							Transaction: &model.Transaction{
								ID: transactionID,
							},
						}
					}
					if err := readWriter.WriteTraceEvent(traceID, transactionID, transaction, wOpts); err != nil {
						b.Fatal(err)
					}
				}

				// NOTE(marclop) We want to check how badly the read performance is affected with
				// by having uncommitted events in the badger TX.
				b.ResetTimer()
				var batch model.Batch
				for i := 0; i < b.N; i++ {
					batch = batch[:0]
					if err := readWriter.ReadTraceEvents(traceID, &batch); err != nil {
						b.Fatal(err)
					}
					if len(batch) != count {
						panic(fmt.Errorf(
							"event count mismatch: expected %d, got %d",
							count, len(batch),
						))
					}
				}
			})
		}
	}

	type testCase struct {
		codec eventstorage.Codec
		name  string
	}
	cases := []testCase{
		{
			name:  "json_codec",
			codec: eventstorage.JSONCodec{},
		},
		{
			// This tests the eventstorage performance without
			// JSON encoding. This would be the theoretical
			// upper limit of what we can achieve with a more
			// efficient codec.
			name:  "nop_codec",
			codec: nopCodec{},
		},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			test(b, tc.codec, false)
		})
		b.Run(tc.name+"_big_tx", func(b *testing.B) {
			test(b, tc.codec, true)
		})
	}
}

func BenchmarkIsTraceSampled(b *testing.B) {
	sampledTraceUUID := uuid.Must(uuid.NewV4())
	unsampledTraceUUID := uuid.Must(uuid.NewV4())
	unknownTraceUUID := uuid.Must(uuid.NewV4())

	// Test with varying numbers of events in the trace.
	db := newBadgerDB(b, badgerOptions)
	store := eventstorage.New(db, eventstorage.JSONCodec{})
	readWriter := store.NewReadWriter()
	defer readWriter.Close()
	wOpts := eventstorage.WriterOpts{
		TTL:                 time.Minute,
		StorageLimitInBytes: 0,
	}

	if err := readWriter.WriteTraceSampled(sampledTraceUUID.String(), true, wOpts); err != nil {
		b.Fatal(err)
	}
	if err := readWriter.WriteTraceSampled(unsampledTraceUUID.String(), false, wOpts); err != nil {
		b.Fatal(err)
	}

	bench := func(name string, traceID string, expectError bool, expectSampled bool) {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sampled, err := readWriter.IsTraceSampled(traceID)
				if expectError {
					if err == nil {
						b.Fatal("expected error")
					}
				} else {
					if err != nil {
						b.Fatal(err)
					}
					if sampled != expectSampled {
						b.Fatalf("expected %v, got %v", expectSampled, sampled)
					}
				}
			}
		})
	}
	bench("sampled", sampledTraceUUID.String(), false, true)
	bench("unsampled", unsampledTraceUUID.String(), false, false)
	bench("unknown", unknownTraceUUID.String(), true, false)
}

type nopCodec struct{}

func (nopCodec) DecodeEvent(data []byte, event *model.APMEvent) error { return nil }
func (nopCodec) EncodeEvent(*model.APMEvent) ([]byte, error)          { return nil, nil }

func makeTransaction(id, traceID string) *model.APMEvent {
	return &model.APMEvent{
		Transaction: &model.Transaction{ID: id},
		Service: model.Service{
			Name:        "myname",
			Version:     "version",
			Environment: "dev",
			Language: model.Language{
				Name: "go", Version: "1.17.11",
			},
			Runtime: model.Runtime{
				Name: "gc",
			},
			Framework: model.Framework{
				Name: "name", Version: "foo",
			},
			Node: model.ServiceNode{
				Name: "serviceNode",
			},
		},
		Processor: model.TransactionProcessor,
		Labels: model.Labels{
			"key": model.LabelValue{
				Value: "value",
			},
			"key2": model.LabelValue{
				Values: []string{"value"},
			},
		},
		DataStream: model.DataStream{
			Namespace: "default",
			Type:      "traces",
			Dataset:   "apm_server",
		},
		Agent: model.Agent{
			Name:    "apm-agent-go",
			Version: "2.1.0",
		},
		Container: model.Container{
			ID:        "someid",
			Name:      "name",
			Runtime:   "runtime",
			ImageName: "ImageName",
			ImageTag:  "latest",
		},
		Process: model.Process{
			Pid:        123,
			Ppid:       newIntP(100),
			Title:      "process title",
			Argv:       []string{"arg1", "arg2", "arg3"},
			Executable: "main.go",
		},
		Host: model.Host{
			OS: model.OS{
				Platform: "ubuntu",
				Type:     "linux",
			},
			Name:         "hostname",
			Hostname:     "hostname.full.domain",
			Architecture: "arm64",
		},
		Trace: model.Trace{ID: traceID},
	}
}

func newIntP(i int) *int {
	return &i
}
