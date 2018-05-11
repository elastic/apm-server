package decoder

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
)

type EntityStreamReader func() (map[string]interface{}, error)
type StreamDecoder func(*http.Request) (EntityStreamReader, error)

type StreamReader struct {
	stream *bufio.Reader
	isEOF  bool
}

func (sr *StreamReader) Read() (map[string]interface{}, error) {
	if sr.isEOF {
		return nil, io.EOF
	}

	buf, err := sr.stream.ReadBytes('\n')
	if err == io.EOF {
		// next call should return io.EOF
		sr.isEOF = true
	} else if err != nil {
		return nil, err
	}

	if len(buf) == 0 {
		return sr.Read()
	}

	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	return DecodeJSONData(tmpreader)
}

func StreamDecodeLimitJSONData(maxSize int64) StreamDecoder {
	return func(req *http.Request) (EntityStreamReader, error) {
		reader, err := getDecompressionReader(req)
		if err != nil {
			return nil, err
		}

		limitedReader := http.MaxBytesReader(nil, reader, maxSize)

		sr := &StreamReader{bufio.NewReader(limitedReader), false}

		return sr.Read, nil
	}
}
