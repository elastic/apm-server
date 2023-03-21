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
	newlineBytes  = []byte("\n")
)

type batch struct {
	metadata []byte
	events   [][]byte
}

// Handler is used to replay a set of stored events to a remote APM Server
// using a ReplayTransport.
type Handler struct {
	config     Config
	writerPool sync.Pool
	batches    []batch
}

type Config struct {
	Path      string
	Transport *Transport
	Storage   fs.FS
	Limiter   *rate.Limiter
}

// New creates a new tracehandler.Handler from a glob expression, a filesystem,
// Transport and an optional rate limitter. If the rate limitter is empty, an
// infinite rate limitter will be used. and The glob expression must match one
// file containing one or more batches.
// The file contents should be in plain text, but the in-memory representation
// will use zlib compression (BestCompression) to avoid compressing the batches
// while benchmarking and optimizing memory usage.
func New(config Config) (*Handler, error) {
	if config.Transport == nil {
		return nil, errors.New("empty transport received")
	}
	if config.Limiter == nil {
		config.Limiter = rate.NewLimiter(rate.Inf, 0)
	}

	h := Handler{
		config: config,
		writerPool: sync.Pool{
			New: func() any {
				pw := &pooledWriter{}
				zw, err := zlib.NewWriterLevel(&pw.buf, zlib.BestSpeed)
				if err != nil {
					panic(err)
				}
				pw.Writer = zw
				return pw
			},
		},
	}
	matches, err := fs.Glob(config.Storage, config.Path)
	if err != nil {
		return nil, err
	}
	for _, path := range matches {
		f, err := config.Storage.Open(path)
		if err != nil {
			return nil, err
		}

		s := bufio.NewScanner(f)
		var current *batch
		for s.Scan() {
			line := s.Bytes()
			if len(line) == 0 {
				continue
			}

			// TODO(marclop): Suppport RUM headers and handle them differently.
			if bytes.HasPrefix(line, rumMetaHeader) {
				return nil, errors.New("rum data support not implemented")
			}

			// Copy the line, as it will be overwritten by the next scan.
			linecopy := make([]byte, len(line))
			copy(linecopy, line)
			line = linecopy

			if isMeta := bytes.HasPrefix(line, metaHeader); isMeta {
				h.batches = append(h.batches, batch{})
				current = &h.batches[len(h.batches)-1]
				current.metadata = linecopy
			} else {
				current.events = append(current.events, linecopy)
			}
		}
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
	// TODO add an option to send events with a steady event rate;
	// if the batch is larger than the event rate, it must then be
	// split into smaller batches.
	if err := h.config.Limiter.WaitN(ctx, len(b.events)); err != nil {
		return 0, err
	}

	w := h.writerPool.Get().(*pooledWriter)
	defer func() {
		w.Reset()
		h.writerPool.Put(w)
	}()
	send := func(ctx context.Context) error {
		if err := w.Close(); err != nil {
			return err
		}
		// NOTE(marclop) RUM event replaying is not yet supported.
		return h.config.Transport.SendV2Events(ctx, &w.buf)
	}
	w.Write(b.metadata)
	w.Write(newlineBytes)
	for _, event := range b.events {
		w.Write(event)
		w.Write(newlineBytes)
	}
	if err := send(ctx); err != nil {
		return 0, err
	}
	return uint(len(b.events)), nil
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

type pooledWriter struct {
	buf bytes.Buffer
	*zlib.Writer
}

func (pw *pooledWriter) Reset() {
	pw.buf.Reset()
	pw.Writer.Reset(&pw.buf)
}
