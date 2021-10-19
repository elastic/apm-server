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

package modelindexer

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	jsoniter "github.com/json-iterator/go"

	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/apm-server/elasticsearch"
)

// NOTE(axw) please avoid introducing apm-server specific details to this code;
// it should eventually be removed, and either contributed to go-elasticsearch
// or replaced by a new go-elasticsearch bulk indexing implementation.
//
// At the time of writing, the go-elasticsearch BulkIndexer implementation
// sends all items to a channel, and multiple persistent worker goroutines will
// receive those items and independently fill up their own buffers. Each one
// will independently flush when their buffer is filled up, or when the flush
// interval elapses. If there are many workers, then this may lead to sparse
// bulk requests.
//
// We take a different approach, where we fill up one bulk request at a time.
// When the buffer is filled up, or the flush interval elapses, we start a new
// goroutine to send the request in the background, with a limit on the number
// of concurrent bulk requests. This way we can ensure bulk requests have the
// maximum possible size, based on configuration and throughput.

type bulkIndexer struct {
	client     elasticsearch.Client
	itemsAdded int
	buf        bytes.Buffer
	respBuf    bytes.Buffer
	resp       elasticsearch.BulkIndexerResponse
	aux        []byte
}

func newBulkIndexer(client elasticsearch.Client) *bulkIndexer {
	return &bulkIndexer{client: client}
}

// BulkIndexer resets b, ready for a new request.
func (b *bulkIndexer) Reset() {
	b.itemsAdded = 0
	b.buf.Reset()
	b.respBuf.Reset()
	b.resp = elasticsearch.BulkIndexerResponse{Items: b.resp.Items[:0]}
}

// Added returns the number of buffered items.
func (b *bulkIndexer) Items() int {
	return b.itemsAdded
}

// Len returns the number of buffered bytes.
func (b *bulkIndexer) Len() int {
	return b.buf.Len()
}

// Add encodes an item in the buffer.
func (b *bulkIndexer) Add(item elasticsearch.BulkIndexerItem) error {
	b.writeMeta(item)
	if _, err := b.buf.ReadFrom(item.Body); err != nil {
		return err
	}
	b.buf.WriteRune('\n')
	b.itemsAdded++
	return nil
}

func (b *bulkIndexer) writeMeta(item elasticsearch.BulkIndexerItem) {
	b.buf.WriteRune('{')
	b.aux = strconv.AppendQuote(b.aux, item.Action)
	b.buf.Write(b.aux)
	b.aux = b.aux[:0]
	b.buf.WriteRune(':')
	b.buf.WriteRune('{')
	if item.DocumentID != "" {
		b.buf.WriteString(`"_id":`)
		b.aux = strconv.AppendQuote(b.aux, item.DocumentID)
		b.buf.Write(b.aux)
		b.aux = b.aux[:0]
	}
	if item.Index != "" {
		if item.DocumentID != "" {
			b.buf.WriteRune(',')
		}
		b.buf.WriteString(`"_index":`)
		b.aux = strconv.AppendQuote(b.aux, item.Index)
		b.buf.Write(b.aux)
		b.aux = b.aux[:0]
	}
	b.buf.WriteRune('}')
	b.buf.WriteRune('}')
	b.buf.WriteRune('\n')
}

// Flush executes a bulk request if there are any items buffered, and clears out the buffer.
func (b *bulkIndexer) Flush(ctx context.Context) (elasticsearch.BulkIndexerResponse, error) {
	if b.itemsAdded == 0 {
		return elasticsearch.BulkIndexerResponse{}, nil
	}

	req := esapi.BulkRequest{Body: &b.buf}
	res, err := req.Do(ctx, b.client)
	if err != nil {
		return elasticsearch.BulkIndexerResponse{}, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return elasticsearch.BulkIndexerResponse{}, fmt.Errorf("flush failed: %s", res.String())
	}

	if _, err := b.respBuf.ReadFrom(res.Body); err != nil {
		return elasticsearch.BulkIndexerResponse{}, err
	}

	iter := jsoniter.ConfigFastest.BorrowIterator(b.respBuf.Bytes())
	defer jsoniter.ConfigFastest.ReturnIterator(iter)

	for iter.Error == nil {
		field := iter.ReadObject()
		if field == "" {
			break
		}
		switch field {
		case "took":
			b.resp.Took = iter.ReadInt()
		case "errors":
			b.resp.HasErrors = iter.ReadBool()
		case "items":
			iter.ReadVal(&b.resp.Items)
		}
	}
	return b.resp, iter.Error
}
