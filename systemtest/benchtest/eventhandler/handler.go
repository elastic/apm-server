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
	"compress/zlib"
	"context"
	"errors"
	"io"
	"io/fs"
	"regexp"
)

const maxScanTokenSize = 300 * 1024 // 300Kb.

var (
	metaHeader    = []byte(`{"metadata":`)
	rumMetaHeader = []byte(`{"m":`)
)

type batch struct {
	r io.ReadSeeker
	// items contains the number of events (minus metadata) in the batch.
	items uint
}

// Handler is used to replay a set of stored events to a remote APM Server
// using a ReplayTransport.
type Handler struct {
	transport    *Transport
	batches      []batch
	warmupEvents uint
}

// New creates a new tracehandler.Handler.
func New(p string, t *Transport, storage fs.FS, warmup uint) (*Handler, error) {
	r, err := regexp.Compile(p)
	if err != nil {
		return nil, err
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
	scannerBuf := make([]byte, maxScanTokenSize)

	h := Handler{
		transport:    t,
		warmupEvents: warmup,
	}
	err = fs.WalkDir(storage, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !r.MatchString(path) {
			return nil
		}

		f, err := storage.Open(path)
		if err != nil {
			return err
		}
		s := bufio.NewScanner(f)
		s.Buffer(scannerBuf, 0)
		var wroteMeta bool
		var scanned uint
		for s.Scan() {
			line := s.Bytes()
			if len(line) == 0 {
				continue
			}
			// TODO(marclop): Suppport RUM headers and handle them differently.
			if bytes.HasPrefix(line, rumMetaHeader) {
				return errors.New("rum data support not implemented")
			}

			if !wroteMeta {
				writer.WriteLine(line)
				wroteMeta = true
				continue
			}
			isMeta := bytes.HasPrefix(line, metaHeader)
			if !isMeta {
				writer.WriteLine(line)
				scanned++
				continue
			}

			// Since the current token is a metadata line, it means that we've
			// read the start of the next batch, because of that, we'll flush,
			// close and copy the compressed contents. After we've ensured that
			// we've written the whole batch to the buffer, we reset it and the
			// zlib.Writer so it can be used for the next iteration.
			if err := writer.FlushClose(); err != nil {
				return err
			}
			h.batches = append(h.batches, batch{
				r:     bytes.NewReader(writer.Bytes()),
				items: scanned,
			})
			writer.Reset()
			writer.WriteLine(line)
			scanned = 0
		}

		if err := writer.FlushClose(); err != nil {
			return err
		}
		h.batches = append(h.batches, batch{
			r:     bytes.NewReader(writer.Bytes()),
			items: scanned,
		})
		buf.Reset()
		writer.Reset()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(h.batches) == 0 {
		return nil, errors.New("eventhandler: regex matched no files, please specify a valid regex")
	}
	return &h, nil
}

// SendBatches sends the loaded trace data to the configured transport. Returns
// the total number of documents sent and any transport errors.
func (h *Handler) SendBatches(ctx context.Context) (uint, error) {
	var sentEvents uint
	sendEvents := func(r io.ReadSeeker, events uint) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			defer r.Seek(0, io.SeekStart)
		}
		// NOTE(marclop) RUM event replaying is not yet supported.
		if err := h.transport.SendV2Events(ctx, r); err != nil {
			return err
		}
		sentEvents += events
		return nil
	}
	for _, batch := range h.batches {
		if err := sendEvents(batch.r, batch.items); err != nil {
			return sentEvents, err
		}
	}
	return sentEvents, nil
}

// WarmUpServer will "warm up" the remote APM Server by sending events until
// the configured threshold is met.
func (h *Handler) WarmUpServer(ctx context.Context, threshold uint) error {
	var events uint
	for events < threshold {
		n, err := h.SendBatches(ctx)
		if err != nil {
			return err
		}
		events += n
	}
	return nil
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

func (w *compressedWriter) FlushClose() error {
	if err := w.zwriter.Flush(); err != nil {
		return err
	}
	return w.zwriter.Close()
}
