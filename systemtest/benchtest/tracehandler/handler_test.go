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

package tracehandler

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTransport struct {
	got      *bytes.Buffer
	received uint
}

func (t *mockTransport) SendStream(_ context.Context, r io.Reader) error {
	if t.got == nil {
		return errors.New("buffer not specified")
	}

	zreader, err := zlib.NewReader(r)
	if err != nil {
		return fmt.Errorf("zlib.NewReader(): %v", err)
	}
	defer zreader.Close()

	scanner := bufio.NewScanner(zreader)
	for scanner.Scan() {
		line := scanner.Bytes()
		isMeta := bytes.HasPrefix(line, metaHeader) ||
			bytes.HasPrefix(line, rumMetaHeader)
		if !isMeta {
			t.received++
		}
		t.got.Write(line)
		t.got.Write([]byte("\n"))
	}
	return nil
}

func newHandler(t *testing.T, dir, expr string) (*Handler, *mockTransport) {
	t.Helper()
	transp := &mockTransport{got: &bytes.Buffer{}}
	storage := os.DirFS(dir)
	h, err := New(expr, transp, storage, 0)
	require.NoError(t, err)
	return h, transp
}

func TestHandlerSendBatches(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, transp := newHandler(t, "testdata", "python.*.ndjson")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.NoError(t, err)

		b, err := ioutil.ReadFile(filepath.Join("testdata", "python-test.ndjson"))
		assert.NoError(t, err)

		assert.Equal(t, string(b), transp.got.String()) // Ensure the contents match.
		assert.Equal(t, uint(32), transp.received)
		assert.Equal(t, uint(32), n)             // Ensure there are 32 events (minus metadata).
		assert.Equal(t, 2, len(handler.batches)) // Ensure there are 2 batches.
	})
	t.Run("cancel-before-sendbatches", func(t *testing.T) {
		handler, transp := newHandler(t, "testdata", "python.*.ndjson")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, transp.received, uint(0))
		assert.Equal(t, n, uint(0))
	})
}

func TestHandlerWarmUp(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h, transp := newHandler(t, "testdata", "python.*.ndjson")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		// Warm up with more events than  saved.
		warmupEvents := uint(1000)
		err := h.WarmUpServer(ctx, warmupEvents)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, transp.received, warmupEvents)
	})
	t.Run("cancel-before-warmup", func(t *testing.T) {
		h, transp := newHandler(t, "testdata", "python.*.ndjson")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := h.WarmUpServer(ctx, 10)
		assert.Error(t, err)
		assert.Equal(t, transp.received, uint(0))
	})
}
