package decoder

import (
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"net/http"

	"io/ioutil"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/utility"
)

type Decoder func(req *http.Request) (map[string]interface{}, error)

func DecodeLimitJSONData(maxSize int64) Decoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		contentType := req.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
		}

		reader := req.Body
		if reader == nil {
			return nil, errors.New("no content")
		}

		switch req.Header.Get("Content-Encoding") {
		case "deflate":
			var err error
			reader, err = zlib.NewReader(reader)
			if err != nil {
				return nil, err
			}

		case "gzip":
			var err error
			reader, err = gzip.NewReader(reader)
			if err != nil {
				return nil, err
			}
		}
		v := make(map[string]interface{})
		if err := json.NewDecoder(http.MaxBytesReader(nil, reader, maxSize)).Decode(&v); err != nil {
			// If we run out of memory, for example
			return nil, errors.Wrap(err, "data read error")
		}
		return v, nil
	}
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
		return map[string]interface{}{
			"ip":         utility.ExtractIP(req),
			"user-agent": req.Header.Get("User-Agent"),
		}
	}
	return augmentData(decoder, "user", augment)
}

func DecodeSystemData(decoder Decoder, enabled bool) Decoder {
	if !enabled {
		return decoder
	}

	augment := func(req *http.Request) map[string]interface{} {
		return map[string]interface{}{"ip": utility.ExtractIP(req)}
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
