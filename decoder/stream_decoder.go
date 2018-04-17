package decoder

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

type EntityStreamReader func() (map[string]interface{}, error)
type V1Decoder func(*http.Request) (map[string]interface{}, error)
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

	log.Println("len(buf), sr.isEOF", len(buf), string(buf), sr.isEOF, err)

	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	return DecodeJSONData(tmpreader)
}

func StreamDecodeLimitJSONData(maxSize int64) StreamDecoder {
	return func(req *http.Request) (EntityStreamReader, error) {
		reader, err := readRequestNDJSONData(maxSize)(req)
		if err != nil {
			return nil, err
		}
		sr := &StreamReader{bufio.NewReader(reader), false}

		return sr.Read, nil
	}
}

// func DecoderStreamAdapter(sd StreamDecoder) Decoder {
// 	return func(req *http.Request) (map[string]interface{}, error) {
// 		var sr *StreamReader
// 		var err error

// 		if sr == nil {
// 			sr, err = sd(req)
// 			if err != nil {
// 				return nil, err
// 			}
// 		}
// 		return sr.Read()
// 	}
// }
