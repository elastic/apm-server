package decoder_test

import (
	"bytes"
	"compress/gzip"
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

func TestDecode(t *testing.T) {
	transactionBytes, err := loader.LoadValidDataAsBytes("transaction")
	assert.Nil(t, err)
	data, err := decoder.DecodeJSONData(ioutil.NopCloser(bytes.NewReader(transactionBytes)))
	assert.Nil(t, err)

	req, err := http.NewRequest("POST", "_", bytes.NewReader(transactionBytes))
	req.Header.Add("Content-Type", "application/json")
	assert.Nil(t, err)

	body, err := decoder.DecodeLimitJSONData(1024 * 1024)(req)
	assert.Nil(t, err)
	assert.Equal(t, data, body)
}

func TestDecodeContentType(t *testing.T) {
	transactionBytes, err := loader.LoadValidDataAsBytes("transaction")
	assert.Nil(t, err)
	data, err := decoder.DecodeJSONData(ioutil.NopCloser(bytes.NewReader(transactionBytes)))
	assert.Nil(t, err)

	req, err := http.NewRequest("POST", "_", bytes.NewReader(transactionBytes))
	req.Header.Add("Content-Type", "application/json; charset=UTF-8")
	assert.Nil(t, err)

	body, err := decoder.DecodeLimitJSONData(1024 * 1024)(req)
	assert.Nil(t, err)
	assert.Equal(t, data, body)
}

func TestDecodeSizeLimit(t *testing.T) {
	minimalValid := func() *http.Request {
		req, err := http.NewRequest("POST", "_", strings.NewReader("{}"))
		assert.Nil(t, err)
		req.Header.Add("Content-Type", "application/json")
		return req
	}

	// just fits
	_, err := decoder.DecodeLimitJSONData(2)(minimalValid())
	assert.Nil(t, err)

	// too large, should not be EOF
	_, err = decoder.DecodeLimitJSONData(1)(minimalValid())
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
	_, err := decoder.DecodeLimitJSONData(40)(gzipRequest(bigDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = decoder.DecodeLimitJSONData(1)(gzipRequest(bigDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)

	// compressed size > uncompressed (edge case)
	tinyData := "{}"
	tinyDataGz := gzipBody(tinyData)
	if len(tinyDataGz) < len(tinyData) {
		t.Fatal("compressed data unexpectedly small")
	}
	/// uncompressed just fits
	_, err = decoder.DecodeLimitJSONData(2)(gzipRequest(tinyDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = decoder.DecodeLimitJSONData(1)(gzipRequest(tinyDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)
}

func TestDecodeSourcemapFormData(t *testing.T) {

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileBytes, err := loader.LoadDataAsBytes("data/valid/sourcemap/bundle.js.map")
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

		transactionBytes, err := loader.LoadValidDataAsBytes("transaction")
		assert.Nil(t, err)
		buffer := bytes.NewReader(transactionBytes)

		req, err := http.NewRequest("POST", "_", buffer)
		req.Header.Add("Content-Type", "application/json")
		req.RemoteAddr = test.remoteAddr
		assert.Nil(t, err)

		body, err := decoder.DecodeSystemData(decoder.DecodeLimitJSONData(1024*1024), test.augment)(req)
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

		transactionBytes, err := loader.LoadValidDataAsBytes("transaction")
		assert.Nil(t, err)
		buffer := bytes.NewReader(transactionBytes)

		req, err := http.NewRequest("POST", "_", buffer)
		req.Header.Add("Content-Type", "application/json")
		req.RemoteAddr = test.remoteAddr
		assert.Nil(t, err)

		body, err := decoder.DecodeUserData(decoder.DecodeLimitJSONData(1024*1024), test.augment)(req)
		assert.Nil(t, err)
		user, hasUser := body["user"].(map[string]interface{})
		assert.Equal(t, test.augment, hasUser)

		_, hasUserAgent := user["user-agent"]
		assert.Equal(t, test.augment, hasUserAgent)

		if test.expectIP == "" {
			assert.NotContains(t, user, "ip")
		} else {
			assert.Equal(t, test.expectIP, user["ip"])
		}
	}
}
