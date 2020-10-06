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

	"github.com/pkg/errors"

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

// CompressedRequestReader returns a reader that will decompress
// the body according to the supplied Content-Encoding header in the request
func CompressedRequestReader(req *http.Request) (io.ReadCloser, error) {
	reader := req.Body
	if reader == nil {
		return nil, errors.New("no content")
	}

	cLen := req.ContentLength
	knownCLen := cLen > -1
	if !knownCLen {
		missingContentLengthCounter.Inc()
	}
	switch req.Header.Get("Content-Encoding") {
	case "deflate":
		if knownCLen {
			deflateLengthAccumulator.Add(cLen)
			deflateCounter.Inc()
		}
		var err error
		reader, err = zlib.NewReader(reader)
		if err != nil {
			return nil, err
		}

	case "gzip":
		if knownCLen {
			gzipLengthAccumulator.Add(cLen)
			gzipCounter.Inc()
		}
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
	default:
		if knownCLen {
			uncompressedLengthAccumulator.Add(cLen)
			uncompressedCounter.Inc()
		}
	}
	readerCounter.Inc()
	return reader, nil
}
