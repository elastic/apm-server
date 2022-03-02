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
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"go.elastic.co/fastjson"

	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/elastic/apm-server/elasticsearch"
)

var (
	esHeader = http.Header{"X-Elastic-Product-Origin": []string{"observability"}}
	newline  = []byte("\n")
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
	jsonw      fastjson.Writer
	gzipw      *gzip.Writer
	copybuf    [32 * 1024]byte
	writer     io.Writer
	buf        bytes.Buffer
	respBuf    bytes.Buffer
	resp       elasticsearch.BulkIndexerResponse
}

func newBulkIndexer(client elasticsearch.Client, compressionLevel int) *bulkIndexer {
	b := &bulkIndexer{client: client}
	if compressionLevel != gzip.NoCompression {
		b.gzipw, _ = gzip.NewWriterLevel(&b.buf, compressionLevel)
		b.writer = b.gzipw
	} else {
		b.writer = &b.buf
	}
	b.Reset()
	return b
}

// BulkIndexer resets b, ready for a new request.
func (b *bulkIndexer) Reset() {
	b.itemsAdded = 0
	b.buf.Reset()
	if b.gzipw != nil {
		b.gzipw.Reset(&b.buf)
	}
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
	if _, err := io.CopyBuffer(b.writer, item.Body, b.copybuf[:]); err != nil {
		return err
	}
	if _, err := b.writer.Write(newline); err != nil {
		return err
	}
	b.itemsAdded++
	return nil
}

func (b *bulkIndexer) writeMeta(item elasticsearch.BulkIndexerItem) {
	b.jsonw.RawByte('{')
	b.jsonw.String(item.Action)
	b.jsonw.RawString(":{")
	if item.DocumentID != "" {
		b.jsonw.RawString(`"_id":`)
		b.jsonw.String(item.DocumentID)
	}
	if item.Index != "" {
		if item.DocumentID != "" {
			b.jsonw.RawByte(',')
		}
		b.jsonw.RawString(`"_index":`)
		b.jsonw.String(item.Index)
	}
	b.jsonw.RawString("}}\n")
	b.writer.Write(b.jsonw.Bytes())
	b.jsonw.Reset()
}

// Flush executes a bulk request if there are any items buffered, and clears out the buffer.
func (b *bulkIndexer) Flush(ctx context.Context) (elasticsearch.BulkIndexerResponse, error) {
	if b.itemsAdded == 0 {
		return elasticsearch.BulkIndexerResponse{}, nil
	}
	if b.gzipw != nil {
		if err := b.gzipw.Flush(); err != nil {
			return elasticsearch.BulkIndexerResponse{}, err
		}
	}

	req := esapi.BulkRequest{Body: &b.buf}
	req.Header = esHeader
	if b.gzipw != nil {
		req.Header.Set("Content-Encoding", "gzip")
	}
	res, err := req.Do(ctx, b.client)
	if err != nil {
		return elasticsearch.BulkIndexerResponse{}, err
	}
	defer res.Body.Close()
	if res.IsError() {
		if res.StatusCode == http.StatusTooManyRequests {
			return elasticsearch.BulkIndexerResponse{}, errorTooManyRequests{res: res}
		}
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
		default:
			iter.Skip()
		}
	}
	return b.resp, errors.Wrap(iter.Error, "error decoding bulk response")
}

type errorTooManyRequests struct {
	res *esapi.Response
}

func (e errorTooManyRequests) Error() string {
	return fmt.Sprintf("flush failed: %s", e.res.String())
}
