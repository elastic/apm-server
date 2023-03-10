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
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"math/bits"
	"math/rand"
	"sync"
	"time"

	"github.com/klauspost/compress/zlib"
	"github.com/tidwall/gjson"
	"go.elastic.co/fastjson"
	"golang.org/x/time/rate"
)

var (
	metaHeader    = []byte(`{"metadata":`)
	rumMetaHeader = []byte(`{"m":`)
	newlineBytes  = []byte("\n")

	// supportedTSFormats lists variations of RFC3339 for supporting
	// different formats for the timezone offset. Copied from apm-data.
	supportedTSFormats = []string{
		"2006-01-02T15:04:05Z07:00", // RFC3339
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05Z07",
	}
)

type batch struct {
	metadata []byte
	events   []event
}

type event struct {
	payload    []byte
	objectType string
	timestamp  time.Time
}

// Handler replays stored events to an APM Server.
//
// It is safe to make concurrent calls to Handler methods.
type Handler struct {
	mu         sync.Mutex // guards config.Rand
	config     Config
	writerPool sync.Pool

	batches      []batch
	minTimestamp time.Time // across all batches
}

// Config holds configuration for Handler.
type Config struct {
	// Path holds the path to a file within Storage, holding recorded
	// event batches for replaying. Path may be a glob, in which case
	// it may match more than one file.
	//
	// The file contents are expected to hold ND-JSON event streams in
	// plain text. These may be transformed, and will be compressed
	// (with zlib.BestSpeed to minimise overhead), during replay.
	Path string

	// Storage holds the fs.FS from which the file specified by Path
	// will read, for extracting batches.
	Storage fs.FS

	// Limiter holds a rate.Limiter for controlling the event rate.
	//
	// If Limiter is nil, an infinite rate limiter will be used.
	Limiter *rate.Limiter

	// Transport holds the Transport that will be used for replaying
	// event batches.
	Transport *Transport

	// Rand, if non-nil, will be used for field randomization.
	//
	// If Rand is nil, then a new Rand will be created, seeded with
	// a value read from crypto/rand. If Rand is supplied (non-nil),
	// it must not be invoked concurrently by any other goroutines.
	Rand *rand.Rand

	// RewriteTimestamps controls whether event timestamps are rewritten
	// during replay.
	//
	// If this is false, then the original timestamps are unmodified.
	//
	// If this is true, then for each call to SendBatches the timestamps
	// be adjusted as follows: first we calculate the smallest timestamp
	// across all of the batches, and then compute the delta between an
	// event's timestamp and the smallest timestamp; this is then added
	// to the current time as recorded when SendBatches is invoked.
	RewriteTimestamps bool

	// RewriteIDs controls whether trace, transaction, span, and error
	// event IDs are rewritten.
	//
	// If this is false, then the original IDs are unmodified.
	//
	// If this is true, then for each call to SendBatches the event IDs
	// will be adjusted as follows: first, we generate a random 64-bit
	// integer. Next, for each event ID whose value is entirely hex
	// digits, we will decode each hex digit to a 4-bit value, XOR it
	// with the next 4 bits in the randon number (rotating left), and
	// then re-encode as a hex digit. Note that we intentionally use a
	// single random value to ensure IDs are randomised consistently,
	// such that event relationships are maintained.
	RewriteIDs bool
}

// New creates a new Handler with config.
func New(config Config) (*Handler, error) {
	if config.Transport == nil {
		return nil, errors.New("empty transport received")
	}
	if config.Limiter == nil {
		config.Limiter = rate.NewLimiter(rate.Inf, 0)
	}
	if config.Rand == nil {
		var rngseed int64
		err := binary.Read(cryptorand.Reader, binary.LittleEndian, &rngseed)
		if err != nil {
			return nil, fmt.Errorf("failed to generate seed for math/rand: %w", err)
		}
		config.Rand = rand.New(rand.NewSource(rngseed))
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
			// if the line is meta, create a new batch
			if isMeta := bytes.HasPrefix(line, metaHeader); isMeta {
				h.batches = append(h.batches, batch{
					metadata: linecopy,
				})
				current = &h.batches[len(h.batches)-1]
			} else {
				event := event{payload: linecopy}
				result := gjson.ParseBytes(linecopy)
				result.ForEach(func(key, value gjson.Result) bool {
					event.objectType = key.Str // lines look like {"span":{...}}
					timestampResult := value.Get("timestamp")
					if timestampResult.Exists() {
						switch timestampResult.Type {
						case gjson.Number:
							us := timestampResult.Int()
							if us >= 0 {
								s := us / 1000000
								ns := (us - (s * 1000000)) * 1000
								event.timestamp = time.Unix(s, ns)
							}
						case gjson.String:
							tstr := timestampResult.Str
							for _, f := range supportedTSFormats {
								if t, err := time.Parse(f, tstr); err == nil {
									event.timestamp = t
									break
								}
							}
						}
					}
					return false
				})
				if !event.timestamp.IsZero() {
					if h.minTimestamp.IsZero() || event.timestamp.Before(h.minTimestamp) {
						h.minTimestamp = event.timestamp
					}
				}
				current.events = append(current.events, event)
			}
		}
	}
	if len(h.batches) == 0 {
		return nil, errors.New("eventhandler: glob matched no files, please specify a valid glob pattern")
	}
	return &h, nil
}

// SendBatches sends the loaded trace data to the configured transport,
// returning the total number of documents sent and any transport errors.
func (h *Handler) SendBatches(ctx context.Context) (int, error) {
	h.mu.Lock()
	randomBits := h.config.Rand.Uint64()
	h.mu.Unlock()

	baseTimestamp := time.Now().UTC()
	var sentEvents int
	for _, batch := range h.batches {
		sent, err := h.sendBatch(ctx, batch, baseTimestamp, randomBits)
		if err != nil {
			return sentEvents, err
		}
		sentEvents += sent
	}
	return sentEvents, nil
}

func (h *Handler) sendBatch(
	ctx context.Context,
	b batch,
	baseTimestamp time.Time,
	randomBits uint64,
) (int, error) {
	// TODO add an option to send events with a steady event rate;
	// if the batch is larger than the event rate, it must then be
	// split into smaller batches.
	burst := h.config.Limiter.Burst()
	if burst == 0 {
		err := h.sendHTTPRequest(ctx, b, baseTimestamp, randomBits)
		if err != nil {
			return 0, err
		}
		return len(b.events), nil
	}

	events := b.events
	remaining := len(b.events)
	for remaining > 0 {
		n := burst
		if remaining < burst {
			n = remaining
		}
		tb := batch{
			metadata: b.metadata,
			events:   events[:n],
		}
		err := h.sendHTTPRequest(ctx, tb, baseTimestamp, randomBits)
		if err != nil {
			return len(b.events) - remaining, err
		}
		events = events[n:]
		remaining -= n
	}
	return len(b.events), nil
}

func (h *Handler) sendHTTPRequest(ctx context.Context, b batch, baseTimestamp time.Time, randomBits uint64) error {
	// if len(b.events) == burst, it will wait for full interval
	// if len(b.events) < burst, it will wait shorter time than interval - interval will be shifted
	// len(b.events) > burst cannot happen
	if err := h.config.Limiter.WaitN(ctx, len(b.events)); err != nil {
		return err
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
		if !h.config.RewriteTimestamps && !h.config.RewriteIDs {
			w.Write(event.payload)
			w.Write(newlineBytes)
			continue
		}
		w.rewriteBuf.RawByte('{')
		w.rewriteBuf.String(event.objectType)
		w.rewriteBuf.RawString(":{")
		gjson.GetBytes(event.payload, event.objectType).ForEach(func(key, value gjson.Result) bool {
			if key.Index > 1 {
				w.rewriteBuf.RawByte(',')
			}
			w.rewriteBuf.RawString(key.Raw)
			w.rewriteBuf.RawByte(':')
			switch key.Str {
			case "timestamp":
				if h.config.RewriteTimestamps && !event.timestamp.IsZero() {
					// We always encode rewritten timestamps as strings,
					// so we don't lose any precision when offsetting by
					// either the base timestamp, or the minimum timestamp
					// across all the batches; string-formatted timestamps
					// may have nanosecond precision.
					offset := event.timestamp.Sub(h.minTimestamp)
					timestamp := baseTimestamp.Add(offset)
					w.rewriteBuf.RawByte('"')
					w.rewriteBuf.Time(timestamp, time.RFC3339Nano)
					w.rewriteBuf.RawByte('"')
				} else {
					w.rewriteBuf.RawString(value.Raw)
				}
			case "id", "parent_id", "trace_id", "transaction_id":
				if h.config.RewriteIDs && randomizeTraceID(&w.idBuf, value.Str, randomBits) {
					w.rewriteBuf.RawByte('"')
					w.rewriteBuf.RawBytes(w.idBuf.Bytes())
					w.rewriteBuf.RawByte('"')
					w.idBuf.Reset()
				} else {
					w.rewriteBuf.RawString(value.Raw)
				}
			default:
				w.rewriteBuf.RawString(value.Raw)
			}
			return true
		})
		w.rewriteBuf.RawString("}}")
		w.Write(w.rewriteBuf.Bytes())
		w.Write(newlineBytes)
		w.rewriteBuf.Reset()
	}
	if err := send(ctx); err != nil {
		return err
	}
	return nil
}

func randomizeTraceID(out *bytes.Buffer, in string, randomBits uint64) bool {
	n := len(in)
	for i := 0; i < n; i++ {
		b := in[i]
		if !((b >= '0' && b <= '9') ||
			(b >= 'a' && b <= 'f') ||
			(b >= 'A' && b <= 'F')) {
			// Not all hex.
			return false
		}
	}

	out.Grow(n)
	for i := 0; i < n; i++ {
		b := in[i]

		var h uint8
		switch {
		case b >= '0' && b <= '9':
			h = b - '0'
		case b >= 'a' && b <= 'f':
			h = 10 + (b - 'a')
		case b >= 'A' && b <= 'F':
			h = 10 + (b - 'A')
		}
		h = (h ^ uint8(randomBits)) & 0x0f
		randomBits = bits.RotateLeft64(randomBits, 4)

		if h < 10 {
			b = '0' + h
		} else {
			b = 'a' + (h - 10)
		}
		out.WriteByte(b)
	}
	return true
}

// WarmUpServer will "warm up" the remote APM Server by sending events until
// the context is cancelled.
func (h *Handler) WarmUpServer(ctx context.Context) error {
	baseTimestamp := time.Now().UTC()
	for {
		for _, batch := range h.batches {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if _, err := h.sendBatch(ctx, batch, baseTimestamp, 0); err != nil {
				return err
			}
		}
	}
}

type pooledWriter struct {
	rewriteBuf fastjson.Writer
	idBuf      bytes.Buffer
	buf        bytes.Buffer
	*zlib.Writer
}

func (pw *pooledWriter) Reset() {
	pw.rewriteBuf.Reset()
	pw.idBuf.Reset()
	pw.buf.Reset()
	pw.Writer.Reset(&pw.buf)
}
