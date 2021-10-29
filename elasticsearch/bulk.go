// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package elasticsearch

import (
	"context"
	"io"
	"time"

	esutilv8 "github.com/elastic/go-elasticsearch/v8/esutil"
)

// BulkIndexer represents a parallel, asynchronous, efficient indexer for Elasticsearch.
//
// This is a subset of the go-elasticsearch/esutil.BulkIndexer interface, suitable for
// use with either a v7 or v8 client.
type BulkIndexer interface {
	// Add adds an item to the indexer. It returns an error when the item cannot be added.
	// Use the OnSuccess and OnFailure callbacks to get the operation result for the item.
	//
	// You must call the Close() method after you're done adding items.
	//
	// It is safe for concurrent use. When it's called from goroutines,
	// they must finish before the call to Close, eg. using sync.WaitGroup.
	Add(context.Context, BulkIndexerItem) error

	// Close waits until all added items are flushed and closes the indexer.
	Close(context.Context) error

	// Stats returns indexer statistics.
	Stats() BulkIndexerStats
}

// BulkIndexerConfig represents configuration of the indexer.
//
// This is a subset of the go-elasticsearch/esutil.BulkIndexerConfig type, suitable for use
// with either a v7 or v8 client.
type BulkIndexerConfig struct {
	NumWorkers    int           // The number of workers. Defaults to runtime.NumCPU().
	FlushBytes    int           // The flush threshold in bytes. Defaults to 5MB.
	FlushInterval time.Duration // The flush threshold as duration. Defaults to 30sec.

	OnError      func(context.Context, error)          // Called for indexer errors.
	OnFlushStart func(context.Context) context.Context // Called when the flush starts.
	OnFlushEnd   func(context.Context)                 // Called when the flush ends.

	// Parameters of the Bulk API.
	Index    string
	Pipeline string
	Timeout  time.Duration
}

// BulkIndexerItem represents an indexer item.
//
// This is a clone of the go-elasticsearch/esutil.BulkIndexerItem type, suitable for use
// with either a v7 or v8 client.
type BulkIndexerItem struct {
	Index           string
	Action          string
	DocumentID      string
	Body            io.Reader
	RetryOnConflict *int

	OnSuccess func(context.Context, BulkIndexerItem, BulkIndexerResponseItem)        // Per item
	OnFailure func(context.Context, BulkIndexerItem, BulkIndexerResponseItem, error) // Per item
}

type (
	BulkIndexerStats        esutilv8.BulkIndexerStats
	BulkIndexerResponse     esutilv8.BulkIndexerResponse
	BulkIndexerResponseItem esutilv8.BulkIndexerResponseItem
)

type v8BulkIndexer struct {
	esutilv8.BulkIndexer
}

func (b v8BulkIndexer) Add(ctx context.Context, item BulkIndexerItem) error {
	itemv8 := esutilv8.BulkIndexerItem{
		Index:           item.Index,
		Action:          item.Action,
		DocumentID:      item.DocumentID,
		Body:            item.Body,
		RetryOnConflict: item.RetryOnConflict,
	}
	if item.OnSuccess != nil {
		itemv8.OnSuccess = func(ctx context.Context, itemv8 esutilv8.BulkIndexerItem, resp esutilv8.BulkIndexerResponseItem) {
			item.OnSuccess(ctx, item, BulkIndexerResponseItem(resp))
		}
	}
	if item.OnFailure != nil {
		itemv8.OnFailure = func(ctx context.Context, itemv8 esutilv8.BulkIndexerItem, resp esutilv8.BulkIndexerResponseItem, err error) {
			item.OnFailure(ctx, item, BulkIndexerResponseItem(resp), err)
		}
	}
	return b.BulkIndexer.Add(ctx, itemv8)
}

func (b v8BulkIndexer) Stats() BulkIndexerStats {
	return BulkIndexerStats(b.BulkIndexer.Stats())
}
