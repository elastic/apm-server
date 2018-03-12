package decoder

import (
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"net/http"

	"io/ioutil"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
)

type Decoder func(req *http.Request) (*processor.Intake, error)

func DecodeLimitJSONData(maxSize int64) Decoder {
	return func(req *http.Request) (*processor.Intake, error) {
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
		maxBytesReader := http.MaxBytesReader(nil, reader, maxSize)
		defer maxBytesReader.Close()
		buf, err := ioutil.ReadAll(maxBytesReader)
		if err != nil {
			// If we run out of memory, for example
			return nil, errors.Wrap(err, "data read error")
		}
		return &processor.Intake{Data: buf}, nil
	}
}

func DecodeSourcemapFormData(req *http.Request) (*processor.Intake, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}

	file, _, err := req.FormFile("sourcemap")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	payload := processor.Intake{
		Data:           buf,
		ServiceName:    req.FormValue("service_name"),
		ServiceVersion: req.FormValue("service_version"),
		BundleFilepath: utility.CleanUrlPath(req.FormValue("bundle_filepath")),
	}

	return &payload, nil
}

func DecodeUserData(decoder Decoder, enabled bool) Decoder {
	if !enabled {
		return decoder
	}

	return func(req *http.Request) (*processor.Intake, error) {
		v, err := decoder(req)
		if err != nil {
			return v, err
		}
		v.UserIP = utility.ExtractIP(req)
		v.UserAgent = req.Header.Get("User-Agent")
		return v, nil
	}
}

func DecodeSystemData(decoder Decoder, enabled bool) Decoder {
	if !enabled {
		return decoder
	}

	return func(req *http.Request) (*processor.Intake, error) {
		v, err := decoder(req)
		if err != nil {
			return v, err
		}
		v.SystemIP = utility.ExtractIP(req)
		return v, nil
	}
}
