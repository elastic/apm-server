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
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/tests/loader"
)

var input = []byte(`{"id":"85925e55b43f4342","system": {"hostname":"prod1.example.com"}}`)

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
	data, err := decoder.DecodeSourcemapFormData(false)(req)
	assert.NoError(t, err)

	assert.Len(t, data, 4)
	assert.Equal(t, "js/bundle_no_mapping.js.map", data["bundle_filepath"])
	assert.Equal(t, "My service", data["service_name"])
	assert.Equal(t, "0.1", data["service_version"])
	assert.NotNil(t, data["sourcemap"].(string))
	assert.Equal(t, len(fileBytes), len(data["sourcemap"].(string)))
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
		body, err := decoder.DecodeUserData(test.augment)(req)
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
