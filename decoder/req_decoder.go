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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/utility"
)

type ReqReader func(req *http.Request) (io.ReadCloser, error)
type ReqDecoder func(req *http.Request) (map[string]interface{}, error)

var (
	decoderMetrics                = monitoring.Default.NewRegistry("apm-server.decoder", monitoring.PublishExpvar)
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

func DecodeJSONData(reader io.Reader) (map[string]interface{}, error) {
	v := make(map[string]interface{})
	d := json.NewDecoder(reader)
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		// If we run out of memory, for example
		return nil, errors.Wrap(err, "data read error")
	}
	return v, nil
}

func DecodeSourcemapFormData(enabled bool) ReqDecoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		contentType := req.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
		}

		file, _, err := req.FormFile("sourcemap")
		if err != nil {
			return nil, err
		}
		defer file.Close()

		sourcemapBytes, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}
		payload := map[string]interface{}{
			"sourcemap":       string(sourcemapBytes),
			"service_name":    req.FormValue("service_name"),
			"service_version": req.FormValue("service_version"),
			"bundle_filepath": utility.CleanUrlPath(req.FormValue("bundle_filepath")),
		}
		systemData, err := DecodeSystemData(enabled)(req)
		if err != nil {
			return systemData, err
		}
		for k, v := range systemData {
			payload[k] = v
		}
		return payload, nil
	}
}

func DecodeUserData(enabled bool) ReqDecoder {
	dec := utility.ManualDecoder{}
	return func(req *http.Request) (map[string]interface{}, error) {
		m := map[string]interface{}{}
		if !enabled {
			return m, nil
		}
		m["user-agent"] = dec.UserAgentHeader(req.Header)
		if ip := utility.ExtractIP(req); ip != nil {
			m["ip"] = ip.String()
		}
		return map[string]interface{}{"user": m}, nil
	}
}

func DecodeSystemData(enabled bool) ReqDecoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		m := map[string]interface{}{}
		if !enabled {
			return m, nil
		}
		if ip := utility.ExtractIP(req); ip != nil {
			utility.InsertInMap(m, "system", map[string]interface{}{
				"ip": ip.String(),
			})
		}
		return m, nil
	}
}
