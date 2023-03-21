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

package eventhandler

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"sync"

	"github.com/klauspost/compress/zlib"
	"golang.org/x/time/rate"
)

var (
	metaHeader    = []byte(`{"metadata":`)
	rumMetaHeader = []byte(`{"m":`)
)

type batch struct {
	bytes []byte
	// items contains the number of events (minus metadata) in the batch.
	items uint
}

// Handler is used to replay a set of stored events to a remote APM Server
// using a ReplayTransport.
type Handler struct {
	transport  *Transport
	limiter    *rate.Limiter
	readerPool *byteReaderPool
	batches    []batch
}

// New creates a new tracehandler.Handler from a glob expression, a filesystem,
// Transport and an optional rate limitter. If the rate limitter is empty, an
// infinite rate limitter will be used. and The glob expression must match one
// file containing one or more batches.
// The file contents should be in plain text, but the in-memory representation
// will use zlib compression (BestCompression) to avoid compressing the batches
// while benchmarking and optimizing memory usage.
func New(p string, t *Transport, storage fs.FS, l *rate.Limiter) (*Handler, error) {
	if t == nil {
		return nil, errors.New("empty transport received")
	}

	var buf bytes.Buffer
	zw, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	writer := compressedWriter{
		buf:     &buf,
		zwriter: zw,
	}
	if l == nil {
		l = rate.NewLimiter(rate.Inf, 0)
	}
	h := Handler{
		transport:  t,
		limiter:    l,
		readerPool: newByteReaderPool(),
	}
	matches, err := fs.Glob(storage, p)
	if err != nil {
		return nil, err
	}
	for _, path := range matches {
		f, err := storage.Open(path)
		if err != nil {
			return nil, err
		}
		s := bufio.NewScanner(f)
		var scanned uint
		for s.Scan() {
			line := s.Bytes()
			if len(line) == 0 {
				continue
			}
			// TODO(marclop): Suppport RUM headers and handle them differently.
			if bytes.HasPrefix(line, rumMetaHeader) {
				return nil, errors.New("rum data support not implemented")
			}

			if isMeta := bytes.HasPrefix(line, metaHeader); !isMeta {
				writer.WriteLine(line)
				scanned++
				continue
			}

			// Since the current token is a metadata line, it means that we've
			// read the start of the batch, because of that, we'll only flush,
			// close and copy the compressed contents when the scanned events
			// are greater than 0. The first iteration will only write the meta
			// and continue decoding events.
			// After writing the whole batch to the buffer, we reset it and the
			// zlib.Writer so it can be used for the next iteration.
			if scanned > 0 {
				if err := writer.Close(); err != nil {
					return nil, err
				}
				h.batches = append(h.batches, batch{
					bytes: writer.Bytes(),
					items: scanned,
				})
			}
			writer.Reset()
			writer.WriteLine(line)
			scanned = 0
		}

		if err := writer.Close(); err != nil {
			return nil, err
		}
		h.batches = append(h.batches, batch{
			bytes: writer.Bytes(),
			items: scanned,
		})
		writer.Reset()
	}
	if len(h.batches) == 0 {
		return nil, errors.New("eventhandler: glob matched no files, please specify a valid glob pattern")
	}
	return &h, nil
}

// SendBatches sends the loaded trace data to the configured transport. Returns
// the total number of documents sent and any transport errors.
func (h *Handler) SendBatches(ctx context.Context) (uint, error) {
	var sentEvents uint
	for _, batch := range h.batches {
		sent, err := h.sendBatch(ctx, batch)
		if err != nil {
			return sentEvents, err
		}
		sentEvents += sent
	}
	return sentEvents, nil
}

func (h *Handler) sendBatch(ctx context.Context, b batch) (uint, error) {
	if err := h.limiter.WaitN(ctx, int(b.items)); err != nil {
		return 0, err
	}
	r := h.readerPool.newReader(b.bytes)
	defer h.readerPool.release(r)
	// NOTE(marclop) RUM event replaying is not yet supported.
	if err := h.transport.SendV2Events(ctx, r); err != nil {
		return 0, err
	}
	return b.items, nil
}

// WarmUpServer will "warm up" the remote APM Server by sending events until
// the context is cancelled.
func (h *Handler) WarmUpServer(ctx context.Context) error {
	for {
		for _, batch := range h.batches {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if _, err := h.sendBatch(ctx, batch); err != nil {
				return err
			}
		}
	}
}

type compressedWriter struct {
	zwriter *zlib.Writer
	buf     *bytes.Buffer
}

func (w *compressedWriter) WriteLine(in []byte) (n int, err error) {
	newline := []byte("\n")
	n, err = w.zwriter.Write(in)
	if err != nil {
		return n, err
	}
	_, err = w.zwriter.Write(newline)
	return n + len(newline), err
}

func (w *compressedWriter) Reset() {
	w.buf.Reset()
	w.zwriter.Reset(w.buf)
}

func (w *compressedWriter) Bytes() []byte {
	contents := w.buf.Bytes()
	tmp := make([]byte, len(contents))
	copy(tmp, contents)
	return tmp
}

func (w *compressedWriter) Close() error {
	return w.zwriter.Close()
}

func newByteReaderPool() *byteReaderPool {
	return &byteReaderPool{p: sync.Pool{New: func() any {
		return bytes.NewReader(nil)
	}}}
}

type byteReaderPool struct {
	p sync.Pool
}

func (bp *byteReaderPool) newReader(b []byte) *bytes.Reader {
	r := bp.p.Get().(*bytes.Reader)
	r.Reset(b)
	return r
}

func (bp *byteReaderPool) release(r *bytes.Reader) {
	bp.p.Put(r)
}
