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

package decoder

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"

	"github.com/elastic/beats/v7/libbeat/monitoring"
)

var (
	decoderMetrics                = monitoring.Default.NewRegistry("apm-server.decoder")
	missingContentLengthCounter   = monitoring.NewInt(decoderMetrics, "missing-content-length.count")
	deflateLengthAccumulator      = monitoring.NewInt(decoderMetrics, "deflate.content-length")
	deflateCounter                = monitoring.NewInt(decoderMetrics, "deflate.count")
	gzipLengthAccumulator         = monitoring.NewInt(decoderMetrics, "gzip.content-length")
	gzipCounter                   = monitoring.NewInt(decoderMetrics, "gzip.count")
	uncompressedLengthAccumulator = monitoring.NewInt(decoderMetrics, "uncompressed.content-length")
	uncompressedCounter           = monitoring.NewInt(decoderMetrics, "uncompressed.count")
	readerCounter                 = monitoring.NewInt(decoderMetrics, "reader.count")
)

const (
	unspecifiedContentEncoding = iota
	deflateContentEncoding
	gzipContentEncoding
	uncompressedContentEncoding
)

// CompressedRequestReader returns a reader that will decompress the body
// according to the supplied Content-Encoding request header, or by sniffing
// the body contents if no header is supplied by looking for magic byte
// headers.
//
// Content-Encoding sniffing is implemented to support the RUM agent sending
// compressed payloads using the Beacon API (https://w3c.github.io/beacon/),
// which does not support specifying request headers.
func CompressedRequestReader(req *http.Request) (io.ReadCloser, error) {
	cLen := req.ContentLength
	knownCLen := cLen > -1
	if !knownCLen {
		missingContentLengthCounter.Inc()
	}

	var reader io.ReadCloser
	var err error
	contentEncoding := unspecifiedContentEncoding
	switch req.Header.Get("Content-Encoding") {
	case "deflate":
		contentEncoding = deflateContentEncoding
		reader, err = zlib.NewReader(req.Body)
	case "gzip":
		contentEncoding = gzipContentEncoding
		reader, err = gzip.NewReader(req.Body)
	default:
		// Sniff encoding from payload by looking at the first two bytes.
		// This produces much less garbage than opportunistically calling
		// gzip.NewReader, zlib.NewReader, etc.
		//
		// Portions of code based on compress/zlib and compress/gzip.
		const (
			zlibDeflate = 8
			gzipID1     = 0x1f
			gzipID2     = 0x8b
		)
		rc := &compressedRequestReadCloser{reader: req.Body, Closer: req.Body}
		if _, err := req.Body.Read(rc.magic[:]); err != nil {
			if err == io.EOF {
				return req.Body, nil
			}
			return nil, err
		}
		if rc.magic[0] == gzipID1 && rc.magic[1] == gzipID2 {
			contentEncoding = gzipContentEncoding
			reader, err = gzip.NewReader(rc)
		} else if rc.magic[0]&0x0f == zlibDeflate {
			contentEncoding = deflateContentEncoding
			reader, err = zlib.NewReader(rc)
		} else {
			contentEncoding = uncompressedContentEncoding
			reader = rc
		}
	}
	if err != nil {
		return nil, err
	}

	switch contentEncoding {
	case deflateContentEncoding:
		if knownCLen {
			deflateLengthAccumulator.Add(cLen)
			deflateCounter.Inc()
		}
	case gzipContentEncoding:
		if knownCLen {
			gzipLengthAccumulator.Add(cLen)
			gzipCounter.Inc()
		}
	case uncompressedContentEncoding:
		if knownCLen {
			uncompressedLengthAccumulator.Add(cLen)
			uncompressedCounter.Inc()
		}
	}
	readerCounter.Inc()
	return reader, nil
}

type compressedRequestReadCloser struct {
	magic     [2]byte
	magicRead int
	reader    io.Reader
	io.Closer
}

func (r *compressedRequestReadCloser) Read(p []byte) (int, error) {
	var nmagic int
	if r.magicRead < 2 {
		nmagic = copy(p[:], r.magic[r.magicRead:])
		r.magicRead += nmagic
	}
	n, err := r.reader.Read(p[nmagic:])
	return n + nmagic, err
}
