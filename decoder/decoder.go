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

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/monitoring"
)

type Reader func(req *http.Request) (io.ReadCloser, error)
type Decoder func(req *http.Request) (map[string]interface{}, error)

var (
	decoderMetrics                = monitoring.Default.NewRegistry("apm-server.decoder", monitoring.PublishExpvar)
	missingContentLengthCounter   = monitoring.NewInt(decoderMetrics, "missing-content-length.count")
	deflateLengthAccumulator      = monitoring.NewInt(decoderMetrics, "deflate.content-length")
	deflateCounter                = monitoring.NewInt(decoderMetrics, "deflate.count")
	gzipLengthAccumulator         = monitoring.NewInt(decoderMetrics, "gzip.content-length")
	gzipCounter                   = monitoring.NewInt(decoderMetrics, "gzip.count")
	uncompressedLengthAccumulator = monitoring.NewInt(decoderMetrics, "uncompressed.content-length")
	uncompressedCounter           = monitoring.NewInt(decoderMetrics, "uncompressed.count")
	readerAccumulator             = monitoring.NewInt(decoderMetrics, "reader.size")
	readerCounter                 = monitoring.NewInt(decoderMetrics, "reader.count")
)

type monitoringReader struct {
	r io.ReadCloser
}

func (mr monitoringReader) Read(p []byte) (int, error) {
	n, err := mr.r.Read(p)
	readerAccumulator.Add(int64(n))
	return n, err
}

func (mr monitoringReader) Close() error {
	return mr.r.Close()
}

func DecodeLimitJSONData(maxSize int64) Decoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		reader, err := readRequestJSONData(maxSize)(req)
		if err != nil {
			return nil, err
		}
		return DecodeJSONData(monitoringReader{reader})
	}
}

// readRequestJSONData makes a function that uses information from an http request to construct a Limited ReadCloser
// of json data from the body of the request
func readRequestJSONData(maxSize int64) Reader {
	return func(req *http.Request) (io.ReadCloser, error) {
		contentType := req.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
		}

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
		return http.MaxBytesReader(nil, reader, maxSize), nil
	}
}

func DecodeJSONData(reader io.ReadCloser) (map[string]interface{}, error) {
	v := make(map[string]interface{})
	d := json.NewDecoder(reader)
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		// If we run out of memory, for example
		return nil, errors.Wrap(err, "data read error")
	}
	return v, nil
}

func DecodeSourcemapFormData(req *http.Request) (map[string]interface{}, error) {
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

	return payload, nil
}

func DecodeUserData(decoder Decoder, enabled bool) Decoder {
	if !enabled {
		return decoder
	}

	augment := func(req *http.Request) map[string]interface{} {
		m := map[string]interface{}{
			"user-agent": req.Header.Get("User-Agent"),
		}
		if ip := utility.ExtractIP(req); ip != nil {
			m["ip"] = ip.String()
		}
		return m
	}
	return augmentData(decoder, "user", augment)
}

func DecodeSystemData(decoder Decoder, enabled bool) Decoder {
	if !enabled {
		return decoder
	}

	augment := func(req *http.Request) map[string]interface{} {
		if ip := utility.ExtractIP(req); ip != nil {
			return map[string]interface{}{"ip": ip.String()}
		}
		return nil
	}
	return augmentData(decoder, "system", augment)
}

func augmentData(decoder Decoder, key string, augment func(req *http.Request) map[string]interface{}) Decoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		v, err := decoder(req)
		if err != nil {
			return v, err
		}
		utility.InsertInMap(v, key, augment(req))
		return v, nil
	}
}
