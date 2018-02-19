package decoder

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"mime/multipart"

	"github.com/elastic/apm-server/tests/loader"
)

func TestDecode(t *testing.T) {
	transactionBytes, err := loader.LoadValidDataAsBytes("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transactionBytes)
	var data map[string]interface{}
	json.Unmarshal(transactionBytes, &data)

	req, err := http.NewRequest("POST", "_", buffer)
	req.Header.Add("Content-Type", "application/json")
	assert.Nil(t, err)

	body, err := DecodeLimitJSONData(1024 * 1024)(req)
	assert.Nil(t, err)
	assert.Equal(t, data, body)
}

func TestDecodeContentType(t *testing.T) {
	transactionBytes, err := loader.LoadValidDataAsBytes("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transactionBytes)
	var data map[string]interface{}
	json.Unmarshal(transactionBytes, &data)

	req, err := http.NewRequest("POST", "_", buffer)
	req.Header.Add("Content-Type", "application/json; charset=UTF-8")
	assert.Nil(t, err)

	body, err := DecodeLimitJSONData(1024 * 1024)(req)
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
	_, err := DecodeLimitJSONData(2)(minimalValid())
	assert.Nil(t, err)

	// too large, should not be EOF
	_, err = DecodeLimitJSONData(1)(minimalValid())
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
	_, err := DecodeLimitJSONData(40)(gzipRequest(bigDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = DecodeLimitJSONData(1)(gzipRequest(bigDataGz))
	assert.NotNil(t, err)
	assert.NotEqual(t, err, io.EOF)

	// compressed size > uncompressed (edge case)
	tinyData := "{}"
	tinyDataGz := gzipBody(tinyData)
	if len(tinyDataGz) < len(tinyData) {
		t.Fatal("compressed data unexpectedly small")
	}
	/// uncompressed just fits
	_, err = DecodeLimitJSONData(2)(gzipRequest(tinyDataGz))
	assert.Nil(t, err)
	/// uncompressed too big
	_, err = DecodeLimitJSONData(1)(gzipRequest(tinyDataGz))
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
	data, err := DecodeSourcemapFormData(req)
	assert.NoError(t, err)

	assert.Len(t, data, 4)
	assert.Equal(t, "js/bundle_no_mapping.js.map", data["bundle_filepath"])
	assert.Equal(t, "My service", data["service_name"])
	assert.Equal(t, "0.1", data["service_version"])
	assert.NotNil(t, data["sourcemap"].(string))
	assert.Equal(t, len(fileBytes), len(data["sourcemap"].(string)))
}

func TestDecodeSystemData(t *testing.T) {
	transactionBytes, err := loader.LoadValidDataAsBytes("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transactionBytes)

	req, err := http.NewRequest("POST", "_", buffer)
	req.Header.Add("Content-Type", "application/json")
	assert.Nil(t, err)

	body, err := DecodeSystemData(DecodeLimitJSONData(1024 * 1024))(req)
	assert.Nil(t, err)
	system, hasSystem := body["system"].(map[string]interface{})
	assert.True(t, hasSystem)
	_, hasIp := system["ip"]
	assert.True(t, hasIp)

}

func TestDecodeUserData(t *testing.T) {
	transactionBytes, err := loader.LoadValidDataAsBytes("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transactionBytes)

	req, err := http.NewRequest("POST", "_", buffer)
	req.Header.Add("Content-Type", "application/json")
	assert.Nil(t, err)

	body, err := DecodeUserData(DecodeLimitJSONData(1024 * 1024))(req)
	assert.Nil(t, err)
	user, hasUser := body["user"].(map[string]interface{})
	assert.True(t, hasUser)
	_, hasIp := user["ip"]
	assert.True(t, hasIp)

	_, hasUserAgent := user["user_agent"]
	assert.True(t, hasUserAgent)

}
