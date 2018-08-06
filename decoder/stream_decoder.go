package decoder

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

func StreamDecodeLimitJSONData(req *http.Request, maxSize int64) (*NDJSONStreamReader, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/x-ndjson") {
		return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}

	reader, err := CompressedRequestReader(maxSize)(req)
	if err != nil {
		return nil, err
	}

	return &NDJSONStreamReader{bufio.NewReader(reader), false}, nil
}

type NDJSONStreamReader struct {
	stream *bufio.Reader
	isEOF  bool
}

func (sr *NDJSONStreamReader) Read() (map[string]interface{}, error) {
	// ReadBytes can return valid data in `buf` _and_ also an io.EOF
	buf, readErr := sr.stream.ReadBytes('\n')
	if readErr != nil && readErr != io.EOF {
		return nil, readErr
	}

	sr.isEOF = readErr == io.EOF

	if len(buf) == 0 {
		return nil, readErr
	}

	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	decoded, err := DecodeJSONData(tmpreader)
	if err != nil {
		return nil, err
	}

	return decoded, readErr // this might be io.EOF
}

func (n *NDJSONStreamReader) SkipToEnd() (uint, error) {
	objects := uint(0)
	nl := []byte("\n")
	var readErr error
	for readErr == nil {
		countBuf := make([]byte, 2048)
		_, readErr = n.stream.Read(countBuf)
		objects += uint(bytes.Count(countBuf, nl))
	}
	return objects, readErr
}

func (n *NDJSONStreamReader) IsEOF() bool {
	return n.isEOF
}
