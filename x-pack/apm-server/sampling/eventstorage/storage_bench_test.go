// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package eventstorage_test

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func BenchmarkWriteTransaction(b *testing.B) {
	test := func(b *testing.B, codec eventstorage.Codec) {
		db := newBadgerDB(b, badgerOptions)
		ttl := time.Minute
		store := eventstorage.New(db, codec, ttl)
		readWriter := store.NewReadWriter()
		defer readWriter.Close()

		traceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		transactionID := []byte{1, 2, 3, 4, 5, 6, 7, 8}
		transaction := &model.Transaction{
			TraceID: hex.EncodeToString(traceID),
			ID:      hex.EncodeToString(transactionID),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := readWriter.WriteTransaction(transaction); err != nil {
				b.Fatal(err)
			}
		}
		assert.NoError(b, readWriter.Flush())
	}
	b.Run("json_codec", func(b *testing.B) {
		test(b, eventstorage.JSONCodec{})
	})
	b.Run("nop_codec", func(b *testing.B) {
		// This tests the eventstorage performance without
		// JSON encoding. This would be the theoretical
		// upper limit of what we can achieve with a more
		// efficient codec.
		test(b, nopCodec{})
	})
}

func BenchmarkReadEvents(b *testing.B) {
	traceUUID := uuid.Must(uuid.NewV4())

	test := func(b *testing.B, codec eventstorage.Codec) {
		// Test with varying numbers of events in the trace.
		counts := []int{0, 1, 10, 100, 1000}
		for _, count := range counts {
			b.Run(fmt.Sprintf("%d events", count), func(b *testing.B) {
				db := newBadgerDB(b, badgerOptions)
				ttl := time.Minute
				store := eventstorage.New(db, codec, ttl)
				readWriter := store.NewReadWriter()
				defer readWriter.Close()

				for i := 0; i < count; i++ {
					transactionUUID := uuid.Must(uuid.NewV4())
					transaction := &model.Transaction{
						TraceID: traceUUID.String(),
						ID:      transactionUUID.String(),
					}
					if err := readWriter.WriteTransaction(transaction); err != nil {
						b.Fatal(err)
					}
				}

				// NOTE(axw) we don't explicitly flush, which is most representative of
				// real workloads. For larger event counts, this ensures we exercise the
				// code path which automatically flushes before reads.

				b.ResetTimer()
				var batch model.Batch
				for i := 0; i < b.N; i++ {
					batch = batch[:0]
					if err := readWriter.ReadEvents(traceUUID.String(), &batch); err != nil {
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

	b.Run("json_codec", func(b *testing.B) {
		test(b, eventstorage.JSONCodec{})
	})
	b.Run("nop_codec", func(b *testing.B) {
		// This tests the eventstorage performance without
		// JSON decoding. This would be the theoretical
		// upper limit of what we can achieve with a more
		// efficient codec.
		test(b, nopCodec{})
	})
}

func BenchmarkIsTraceSampled(b *testing.B) {
	sampledTraceUUID := uuid.Must(uuid.NewV4())
	unsampledTraceUUID := uuid.Must(uuid.NewV4())
	unknownTraceUUID := uuid.Must(uuid.NewV4())

	// Test with varying numbers of events in the trace.
	db := newBadgerDB(b, badgerOptions)
	ttl := time.Minute
	store := eventstorage.New(db, eventstorage.JSONCodec{}, ttl)
	readWriter := store.NewReadWriter()
	defer readWriter.Close()

	if err := readWriter.WriteTraceSampled(sampledTraceUUID.String(), true); err != nil {
		b.Fatal(err)
	}
	if err := readWriter.WriteTraceSampled(unsampledTraceUUID.String(), false); err != nil {
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

func (nopCodec) DecodeSpan(data []byte, span *model.Span) error             { return nil }
func (nopCodec) DecodeTransaction(data []byte, tx *model.Transaction) error { return nil }
func (nopCodec) EncodeSpan(*model.Span) ([]byte, error)                     { return nil, nil }
func (nopCodec) EncodeTransaction(*model.Transaction) ([]byte, error)       { return nil, nil }
