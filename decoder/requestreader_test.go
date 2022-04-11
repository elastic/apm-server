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

package decoder_test

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
)

func TestCompressedRequestReader(t *testing.T) {
	uncompressed := "uncompressed input"
	zlibCompressed := zlibCompressString(uncompressed)
	gzipCompressed := gzipCompressString(uncompressed)

	requestBodyReader := func(contentEncoding string, body []byte) (io.ReadCloser, error) {
		req := httptest.NewRequest("GET", "/", bytes.NewReader(body))
		if contentEncoding != "" {
			req.Header.Set("Content-Encoding", contentEncoding)
		}
		return decoder.CompressedRequestReader(req)
	}

	type test struct {
		input           []byte
		contentEncoding string
	}
	for _, test := range []test{{
		input:           []byte(uncompressed),
		contentEncoding: "", // sniff
	}, {
		input:           zlibCompressed,
		contentEncoding: "", // sniff
	}, {
		input:           gzipCompressed,
		contentEncoding: "", // sniff
	}, {
		input:           zlibCompressed,
		contentEncoding: "deflate",
	}, {
		input:           gzipCompressed,
		contentEncoding: "gzip",
	}} {
		reader, err := requestBodyReader(test.contentEncoding, test.input)
		require.NoError(t, err)
		assertReaderContents(t, uncompressed, reader)
	}

	_, err := requestBodyReader("deflate", gzipCompressed)
	assert.Equal(t, zlib.ErrHeader, err)

	_, err = requestBodyReader("gzip", zlibCompressed)
	assert.Equal(t, gzip.ErrHeader, err)
}

func TestCompressedRequestReaderEmpty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	reader, err := decoder.CompressedRequestReader(req)
	require.NoError(t, err)

	data, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Empty(t, data)
}

func BenchmarkCompressedRequestReader(b *testing.B) {
	benchmark := func(b *testing.B, input []byte, contentEncoding string) {
		req := httptest.NewRequest("GET", "/", bytes.NewReader(input))
		if contentEncoding != "" {
			req.Header.Set("Content-Encoding", contentEncoding)
		}
		for i := 0; i < b.N; i++ {
			req.Body = ioutil.NopCloser(bytes.NewReader(input))
			if _, err := decoder.CompressedRequestReader(req); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.Run("uncompressed", func(b *testing.B) {
		benchmark(b, []byte("uncompressed"), "")
	})
	b.Run("gzip_content_encoding", func(b *testing.B) {
		benchmark(b, gzipCompressString("uncompressed"), "gzip")
	})
	b.Run("gzip_sniff", func(b *testing.B) {
		benchmark(b, gzipCompressString("uncompressed"), "")
	})
	b.Run("deflate_content_encoding", func(b *testing.B) {
		benchmark(b, zlibCompressString("uncompressed"), "deflate")
	})
	b.Run("deflate_sniff", func(b *testing.B) {
		benchmark(b, zlibCompressString("uncompressed"), "")
	})
}

func assertReaderContents(t *testing.T, expected string, r io.Reader) {
	t.Helper()
	contents, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, expected, string(contents))
}

func zlibCompressString(s string) []byte {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestSpeed)
	if err != nil {
		panic(err)
	}
	compressString(s, w)
	return buf.Bytes()
}

func gzipCompressString(s string) []byte {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		panic(err)
	}
	compressString(s, w)
	return buf.Bytes()
}

func compressString(s string, w io.WriteCloser) {
	if _, err := w.Write([]byte(s)); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
}
