// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling_test

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
)

func BenchmarkProcess(b *testing.B) {
	processor, err := sampling.NewProcessor(newTempdirConfig(b))
	require.NoError(b, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	b.RunParallel(func(pb *testing.PB) {
		var seed int64
		err := binary.Read(cryptorand.Reader, binary.LittleEndian, &seed)
		assert.NoError(b, err)
		rng := rand.New(rand.NewSource(seed))

		var traceID [16]byte
		for pb.Next() {
			binary.LittleEndian.PutUint64(traceID[:8], rng.Uint64())
			binary.LittleEndian.PutUint64(traceID[8:], rng.Uint64())
			transactionID := traceID[:8]
			spanID := traceID[8:]
			trace := model.Trace{ID: hex.EncodeToString(traceID[:])}
			transaction := &model.Transaction{
				ID: hex.EncodeToString(transactionID),
			}
			spanParentID := hex.EncodeToString(transactionID)
			span := &model.Span{
				ID:       hex.EncodeToString(spanID),
				ParentID: spanParentID,
			}
			batch := model.Batch{
				{Trace: trace, Transaction: transaction},
				{Trace: trace, Span: span},
				{Trace: trace, Span: span},
				{Trace: trace, Span: span},
			}
			if err := processor.ProcessBatch(context.Background(), &batch); err != nil {
				b.Fatal(err)
			}
		}
	})
}
