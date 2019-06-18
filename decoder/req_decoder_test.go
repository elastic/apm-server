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

package decoder_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/tests/loader"
)

var input = []byte(`{"id":"85925e55b43f4342","system": {"hostname":"prod1.example.com"}}`)

func TestDecode(t *testing.T) {
	data, err := decoder.DecodeJSONData(ioutil.NopCloser(bytes.NewReader(input)))
	assert.Nil(t, err)
	assert.Equal(t, data, map[string]interface{}{
		"id":     "85925e55b43f4342",
		"system": map[string]interface{}{"hostname": "prod1.example.com"}})
}

func TestDecodeSizeLimit(t *testing.T) {
	minimalValid := func() *http.Request {
		req, err := http.NewRequest("POST", "_", strings.NewReader("{}"))
		assert.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")
		return req
	}

	// just fits
	_, err := decodeLimitJSONData(2)(minimalValid())
	assert.Nil(t, err)

	// too large, should not be EOF
	_, err = decodeLimitJSONData(1)(minimalValid())
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)
}

func TestDecodeSizeLimitGzip(t *testing.T) {
	gzipBody := func(body string) []byte {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write([]byte("{}"))
		assert.Nil(t, err)
		err = zw.Close()
		assert.Nil(t, err)
		return buf.Bytes()
	}
	gzipRequest := func(body []byte) *http.Request {
		req, err := http.NewRequest("POST", "_", bytes.NewReader(body))
		assert.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Content-Encoding", "gzip")
		return req
	}

	// compressed size < uncompressed (usual case)
	bigData := `{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": 1}`
	bigDataGz := gzipBody(bigData)
	if len(bigDataGz) > len(bigData) {
		t.Fatal("compressed data unexpectedly big")
	}
	/// uncompressed just fits
	_, err := decodeLimitJSONData(40)(gzipRequest(bigDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = decodeLimitJSONData(1)(gzipRequest(bigDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)

	// compressed size > uncompressed (edge case)
	tinyData := "{}"
	tinyDataGz := gzipBody(tinyData)
	if len(tinyDataGz) < len(tinyData) {
		t.Fatal("compressed data unexpectedly small")
	}
	/// uncompressed just fits
	_, err = decodeLimitJSONData(2)(gzipRequest(tinyDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = decodeLimitJSONData(1)(gzipRequest(tinyDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)
}

func TestDecodeSourcemapFormData(t *testing.T) {

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileBytes, err := loader.LoadDataAsBytes("../testdata/sourcemap/bundle.js.map")
	assert.NoError(t, err)
	part, err := writer.CreateFormFile("sourcemap", "bundle_no_mapping.js.map")
	assert.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(fileBytes))
	assert.NoError(t, err)

	writer.WriteField("bundle_filepath", "js/./test/../bundle_no_mapping.js.map")
	writer.WriteField("service_name", "My service")
	writer.WriteField("service_version", "0.1")

	err = writer.Close()
	assert.NoError(t, err)

	req, err := http.NewRequest("POST", "_", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	assert.NoError(t, err)

	assert.NoError(t, err)
	data, err := decoder.DecodeSourcemapFormData(req)
	assert.NoError(t, err)

	assert.Len(t, data, 4)
	assert.Equal(t, "js/bundle_no_mapping.js.map", data["bundle_filepath"])
	assert.Equal(t, "My service", data["service_name"])
	assert.Equal(t, "0.1", data["service_version"])
	assert.NotNil(t, data["sourcemap"].(string))
	assert.Equal(t, len(fileBytes), len(data["sourcemap"].(string)))
}

func TestDecodeSystemData(t *testing.T) {

	type test struct {
		augment    bool
		remoteAddr string
		expectIP   string
	}

	tests := []test{
		{augment: false, remoteAddr: "1.2.3.4:1234"},
		{augment: true, remoteAddr: "1.2.3.4:1234", expectIP: "1.2.3.4"},
		{augment: true, remoteAddr: "not-an-ip:1234"},
		{augment: true, remoteAddr: ""},
	}

	for _, test := range tests {

		buffer := bytes.NewReader(input)

		req, err := http.NewRequest("POST", "_", buffer)
		req.Header.Add("Content-Type", "application/json")
		req.RemoteAddr = test.remoteAddr
		assert.Nil(t, err)

		body, err := decoder.DecodeSystemData(decodeLimitJSONData(1024*1024), test.augment)(req)
		assert.Nil(t, err)

		system, hasSystem := body["system"].(map[string]interface{})
		assert.True(t, hasSystem)

		if test.expectIP == "" {
			assert.NotContains(t, system, "ip")
		} else {
			assert.Equal(t, test.expectIP, system["ip"])
		}
	}
}

func TestDecodeUserData(t *testing.T) {

	type test struct {
		augment         bool
		remoteAddr      string
		userAgent       []string
		expectIP        string
		expectUserAgent string
	}
	ua1 := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36"
	ua2 := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.14; rv:67.0) Gecko/20100101 Firefox/67.0"

	tests := []test{
		{augment: false, remoteAddr: "1.2.3.4:1234", userAgent: []string{ua1}},
		{augment: true, remoteAddr: "1.2.3.4:1234", expectIP: "1.2.3.4", userAgent: []string{ua1, ua2}, expectUserAgent: fmt.Sprintf("%s, %s", ua1, ua2)},
		{augment: true, remoteAddr: "not-an-ip:1234", userAgent: []string{ua1}, expectUserAgent: ua1},
		{augment: true, remoteAddr: ""},
	}

	for _, test := range tests {
		// request setup
		buffer := bytes.NewReader(input)
		req, err := http.NewRequest("POST", "_", buffer)
		req.Header.Add("Content-Type", "application/json")
		req.RemoteAddr = test.remoteAddr
		for _, ua := range test.userAgent {
			req.Header.Add("User-Agent", ua)
		}
		assert.Nil(t, err)

		// decode user data from request
		body, err := decoder.DecodeUserData(decodeLimitJSONData(1024*1024), test.augment)(req)
		assert.Nil(t, err)
		user, hasUser := body["user"].(map[string]interface{})
		assert.Equal(t, test.augment, hasUser)

		if test.expectIP == "" {
			assert.NotContains(t, user, "ip")
		} else {
			assert.Equal(t, test.expectIP, user["ip"])
		}

		if test.expectUserAgent != "" {
			userAgent, ok := user["user-agent"].(string)
			assert.True(t, ok)
			assert.Equal(t, test.expectUserAgent, userAgent)
		}
	}
}

func decodeLimitJSONData(maxSize int64) decoder.ReqDecoder {
	return func(req *http.Request) (map[string]interface{}, error) {
		contentType := req.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
		}

		reader, err := decoder.CompressedRequestReader(req)
		if err != nil {
			return nil, err
		}
		reader = http.MaxBytesReader(nil, reader, maxSize)
		return decoder.DecodeJSONData(reader)
	}
}
