package decoder

import (
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

type Decoder func(req *http.Request) (map[string]interface{}, error)

func DecodeLimitJSONData(maxSize int64) Decoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		contentType := req.Header.Get("Content-Type")
		if contentType != "application/json" {
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
